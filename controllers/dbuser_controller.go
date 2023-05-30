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
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kindarocksv1beta1 "github.com/db-operator/db-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

// DBUserReconciler reconciles a DBUser object
type DBUserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Interval time.Duration
	Log      logr.Logger
}

const (
	dbUserPhaseCreate            = "Creating"
	dbUserPhaseSecretsTemplating = "SecretsTemplating"
	dbUserPhaseFinish            = "Finishing"
	dbUserPhaseReady             = "Ready"
	dbUserPhaseDelete            = "Deleting"
)

//+kubebuilder:rbac:groups=kinda.rocks,resources=dbusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kinda.rocks,resources=dbusers/finalizers,verbs=update

func (r *DBUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("dbuser", req.NamespacedName)

	reconcilePeriod := r.Interval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	dbucr := &kindarocksv1beta1.DBUser{}
	err := r.Get(ctx, req.NamespacedName, dbucr)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Requested object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcileResult, nil
		}
		// Error reading the object - requeue the request.
		return reconcileResult, err
	}

	// Check if DBUser is marked to be deleted
	if dbucr.GetDeletionTimestamp() != nil {
		dbucr.Status.Phase = dbUserPhaseDelete
		if containsString(dbucr.ObjectMeta.Finalizers, "dbuser."+dbucr.Name) {
			if err := r.deleteDBUser(ctx, dbucr); err != nil {
				logrus.Errorf("DB: namespace=%s, name=%s failed deleting database - %s", dbucr.Namespace, dbucr.Name, err)
			}

		}
	}

	// Update object status always when function exit abnormally or through a panic.
	defer func() {
		if err := r.Status().Update(ctx, dbucr); err != nil {
			logrus.Errorf("failed to update status - %s", err)
		}
	}()

	return reconcileResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DBUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindarocksv1beta1.DBUser{}).
		Complete(r)
}

func (r *DBUserReconciler) deleteDBUser(context.Context, DBUserReconciler) error {
	return nil
}
