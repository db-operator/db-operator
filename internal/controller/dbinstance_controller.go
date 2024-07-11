/*
 * Copyright 2021 kloeckner.i GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	kindav1beta2 "github.com/db-operator/db-operator/api/v1beta2"
	kubehelper "github.com/db-operator/db-operator/internal/helpers/kube"
	"github.com/db-operator/db-operator/pkg/config"
	"github.com/db-operator/db-operator/pkg/utils/database"
	kcidb "github.com/db-operator/db-operator/pkg/utils/database"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DbInstanceReconciler reconciles a DbInstance object
type DbInstanceReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Interval   time.Duration
	Recorder   record.EventRecorder
	Conf       *config.Config
	kubeHelper *kubehelper.KubeHelper
}

// +kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbinstances/finalizers,verbs=update

/*
 * Database instance is a connector between the db-operator and databases. When a database instance is
 * created, db-operator should try connectiong to an instance and as an admin user. Once it's done,
 * DbInstance should be marked as ready and get ready for accepting Database resources
 */
func (r *DbInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	reconcilePeriod := r.Interval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Fetch the DbInstance custom resource
	dbin := &kindav1beta2.DbInstance{}
	err := r.Get(ctx, req.NamespacedName, dbin)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Requested object not found, it could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcileResult, nil
		}
		// Error reading the object - requeue the request.
		return reconcileResult, err
	}

	// Update object status always when function returns, either normally or through a panic.
	defer func() {
		if err := r.Status().Update(ctx, dbin); err != nil {
			log.Error(err, "failed to update status")
		}
	}()

	// Kubehelper should be used for all interractions with the k8s api
	r.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbin)

	if err := r.checkConnection(ctx, dbin); err != nil {
		return reconcileResult, err
	}

	// TODO: Check connections
	// TODO: (Probably) Update databases that are connected to this instance
	// if !dbin.Status.Connected {
	// err = r.create(ctx, dbin)
	// if err != nil {
	// log.Error(err, "instance creation failed")
	// return reconcileResult, nil // failed but don't requeue the request. retry by changing spec or config
	// }
	// dbin.Status.Status = true
	// dbin.Status.Phase = dbInstancePhaseBroadcast
	// }
	return reconcileResult, nil
}

// Check whether db-operator is able to connect to the database server
func (r *DbInstanceReconciler) checkConnection(ctx context.Context, dbin *kindav1beta2.DbInstance) (err error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Trying to connect to the database server")

	var host string
	var port uint16

	if from := dbin.Spec.InstanceData.HostFrom; from != nil {
		host, err = r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
		if err != nil {
			return err
		}
	} else {
		host = dbin.Spec.InstanceData.Host
	}

	if from := dbin.Spec.InstanceData.PortFrom; from != nil {
		portStr, err := r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
		if err != nil {
			return err
		}
		port64, err := strconv.ParseUint(portStr, 10, 64)
		if err != nil {
			return err
		}
		port = uint16(port64)
	} else {
		port = dbin.Spec.InstanceData.Port
	}

	db, err := makeInterface(
		string(dbin.Spec.Engine),
		host,
		port,
		dbin.Spec.SSLConnection.Enabled,
		dbin.Spec.SSLConnection.SkipVerify,
	)
	if err != nil {
		return
	}
	from := dbin.Spec.AdminCredentials.UsernameFrom
	username, err := r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
	if err != nil {
		return
	}

	from = dbin.Spec.AdminCredentials.PasswordFrom
	password, err := r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
	if err != nil {
		return
	}

	dbuser := &database.DatabaseUser{
		Username: username,
		Password: password,
	}

	if err = db.CheckStatus(ctx, dbuser); err != nil {
		return
	}

	return nil
}

// TODO: Remove this function and start using the InstanceData database package
// I've decided not to change this code while upgrading the API, because it
// would require rewrite the whole database helper, and it's a big change
// even without it
func makeInterface(engine, host string, port uint16, sslEnabled, skipCAVerify bool) (kcidb.Database, error) {
	switch engine {
	case "postgres":
		db := kcidb.Postgres{
			Host:         host,
			Port:         port,
			Database:     "postgres",
			SSLEnabled:   sslEnabled,
			SkipCAVerify: skipCAVerify,
		}
		return db, nil
	case "mysql":
		db := kcidb.Mysql{
			Host:         host,
			Port:         port,
			Database:     "mysql",
			SSLEnabled:   sslEnabled,
			SkipCAVerify: skipCAVerify,
		}
		return db, nil
	default:
		return nil, fmt.Errorf("not supported engine type: %s", engine)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindav1beta2.DbInstance{}).
		Complete(r)
}

// TODO: implement it separately, it might be a bigger change that affects all the resources
// Broadcast should notify all the databases and dbusers that a dbinstance was updated, and
// they should be reconciled
// func (r *DbInstanceReconciler) broadcast(ctx context.Context, dbin *kindav1beta2.DbInstance) error {
// 	dbList := &kindav1beta2.DatabaseList{}
// 	err := r.List(ctx, dbList)
// 	if err != nil {
// 		return err
// 	}

// 	for _, db := range dbList.Items {
// 		if db.Spec.Instance == dbin.Name {
// 			annotations := db.ObjectMeta.GetAnnotations()
// 			if _, found := annotations["checksum/spec"]; found {
// 				annotations["checksum/spec"] = ""
// 				db.ObjectMeta.SetAnnotations(annotations)
// 				err = r.Update(ctx, &db)
// 				if err != nil {
// 					return err
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }
