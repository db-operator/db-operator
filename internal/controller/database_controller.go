/*
 * Copyright 2021 kloeckner.i GmbH
 * Copyright 2023 DB-Operator Authors
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
	"errors"
	"fmt"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kindav1beta2 "github.com/db-operator/db-operator/api/v1beta2"
	commonhelper "github.com/db-operator/db-operator/internal/helpers/common"
	dbhelper "github.com/db-operator/db-operator/internal/helpers/database"
	kubehelper "github.com/db-operator/db-operator/internal/helpers/kube"
	"github.com/db-operator/db-operator/internal/utils/templates"
	"github.com/db-operator/db-operator/pkg/config"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/db-operator/db-operator/pkg/utils/database"
	"github.com/db-operator/db-operator/pkg/utils/kci"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	Interval        time.Duration
	Conf            *config.Config
	WatchNamespaces []string
	CheckChanges    bool
	kubeHelper      *kubehelper.KubeHelper
}

var (
	dbPhaseReconcile      = "Reconciling"
	dbPhaseCreateOrUpdate = "CreatingOrUpdating"
	dbPhaseConfigMap      = "InfoConfigMapCreating"
	dbPhaseTemplating     = "Templating"
	dbPhaseFinish         = "Finishing"
	dbPhaseReady          = "Ready"
	dbPhaseDelete         = "Deleting"
)

//+kubebuilder:rbac:groups=kinda.rocks,resources=databases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kinda.rocks,resources=databases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kinda.rocks,resources=databases/finalizers,verbs=update

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	phase := dbPhaseReconcile
	log := log.FromContext(ctx)
	reconcilePeriod := r.Interval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Fetch the Database custom resource
	dbcr := &kindav1beta2.Database{}
	err := r.Get(ctx, req.NamespacedName, dbcr)
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

	// Update object status always when function exit abnormally or through a panic.
	defer func() {
		if err := r.Status().Update(ctx, dbcr); err != nil {
			log.Error(err, "failed to update status")
		}
	}()

	r.Recorder.Event(dbcr, "Normal", phase, "Started reconciling db-operator managed database")
	promDBsStatus.WithLabelValues(dbcr.Namespace, dbcr.Spec.Instance, dbcr.Name).Set(boolToFloat64(dbcr.Status.Status))

	// Init the kubehelper object
	r.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbcr)
	/* ----------------------------------------------------------------
	 * -- Check if the Database is marked to be deleted, which is
	 * --  indicated by the deletion timestamp being set.
	 * ------------------------------------------------------------- */
	if dbcr.IsDeleted() {
		return r.handleDbDelete(ctx, dbcr)
	}

	/* ----------------------------------------------------------------
	 * -- If db can't be accessed, set the status to false.
	 * -- It doesn't make sense to run healthCheck if InstanceRef
	 * --  is not set, because it means that database initialization
	 * --  wasn't triggered.
	 * ------------------------------------------------------------- */
	if dbcr.Status.Engine != "" {
		if err := r.healthCheck(ctx, dbcr); err != nil {
			log.Info("Healthcheck is failed")
			dbcr.Status.Status = false
			err = r.Status().Update(ctx, dbcr)
			if err != nil {
				log.Error(err, "error status subresource updating")
				return r.manageError(ctx, dbcr, err, true, phase)
			}
		}
	}

	/* ----------------------------------------------------------------
	 * -- Check if db-operator must fire sql actions, or just
	 * --  update the k8s resources
	 * ------------------------------------------------------------- */
	mustReconile, err := r.isFullReconcile(ctx, dbcr)
	if err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	return r.handleDbCreateOrUpdate(ctx, dbcr, mustReconile)
}

// Move it to helpers and start testing it
func (r *DatabaseReconciler) healthCheck(ctx context.Context, dbcr *kindav1beta2.Database) error {
	var dbSecret *corev1.Secret
	dbSecret, err := r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		return err
	}

	databaseCred, err := dbhelper.ParseDatabaseSecretData(dbcr, dbSecret.Data)
	if err != nil {
		// failed to parse database credential from secret
		return err
	}

	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}

	if err := db.CheckStatus(ctx, dbuser); err != nil {
		return err
	}

	return nil
}

/* --------------------------------------------------------------------
 * -- isFullReconcile returns true if db-operator should fire database
 * --  queries. If user doesn't want to load database server with
 * --  with db-operator queries, there is an option to execute them
 * --  only when db-operator detects the change between the wished
 * --  config and the existing one, or the special annotation is set.
 * -- Otherwise, if user doesn't care about the load, since it
 * --  actually should be very low, the checkForChanges flag can
 * --  be set to false, and then db-operator will run queries
 * --  against the database.
 * ----------------------------------------------------------------- */
func (r *DatabaseReconciler) isFullReconcile(ctx context.Context, dbcr *kindav1beta2.Database) (bool, error) {
	log := log.FromContext(ctx)
	// This is the first check, because even if the checkForChanges is false,
	// the annotation is exptected to be removed
	if _, ok := dbcr.GetAnnotations()[consts.DATABASE_FORCE_FULL_RECONCILE]; ok {
		r.Recorder.Event(dbcr, "Normal", fmt.Sprintf("%s annotation was found", consts.DATABASE_FORCE_FULL_RECONCILE),
			"The full reconciliation cyclce will be executed and the annotation will be removed",
		)

		annotations := dbcr.GetAnnotations()
		delete(annotations, consts.DATABASE_FORCE_FULL_RECONCILE)
		err := r.Update(ctx, dbcr)
		if err != nil {
			log.Error(err, "error resource updating")
			return false, err
		}
		return true, nil
	}

	// If we don't check changes, then just always reconcile
	if !r.CheckChanges {
		return true, nil
	}

	// If database is not healthy, reconcile
	if !dbcr.Status.Status {
		return true, nil
	}

	var dbSecret *corev1.Secret
	var err error

	dbSecret, err = r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return true, nil
		} else {
			return false, err
		}
	}

	return commonhelper.IsDBChanged(dbcr, dbSecret), nil
}

/* --------------------------------------------------------------------
 * -- This function should be called when a database is created
 * --  or updated.
 * -- The mustReconcile argument is the trigger for the database
 * --  action to run. If mustReconcile is true, all the db queries
 * --  will be executed.
 * ------------------------------------------------------------------ */
func (r *DatabaseReconciler) handleDbCreateOrUpdate(ctx context.Context, dbcr *kindav1beta2.Database, mustReconcile bool) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	var err error

	phase := dbPhaseCreateOrUpdate
	r.Recorder.Event(dbcr, "Normal", phase, "Starting process to create or update databases")

	reconcilePeriod := r.Interval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	log.Info("reconciling database")

	defer promDBsPhaseTime.WithLabelValues(phase).Observe(kci.TimeTrack(time.Now()))

	// Handle the secret creation
	var dbSecret *corev1.Secret
	dbSecret, err = r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if err := r.setEngine(ctx, dbcr); err != nil {
				return r.manageError(ctx, dbcr, err, true, phase)
			}
			dbSecret, err = r.createSecret(ctx, dbcr)
			if err != nil {
				return r.manageError(ctx, dbcr, err, false, phase)
			}
		} else {
			return r.manageError(ctx, dbcr, err, false, phase)
		}
	}

	if err := r.kubeHelper.ModifyObject(ctx, dbSecret); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	// Create
	if mustReconcile {
		if err := r.setEngine(ctx, dbcr); err != nil {
			return r.manageError(ctx, dbcr, err, false, phase)
		}

		if err := r.createDatabase(ctx, dbcr, dbSecret); err != nil {
			// when database creation failed, don't requeue request. to prevent exceeding api limit (ex: against google api)
			return r.manageError(ctx, dbcr, err, false, phase)
		}
	}

	phase = dbPhaseConfigMap
	r.Recorder.Event(dbcr, "Normal", phase, "Creating ConfigMap")
	if err = r.handleInfoConfigMap(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}
	phase = dbPhaseTemplating
	r.Recorder.Event(dbcr, "Normal", phase, "Handle templated credentials")

	if err := r.handleTemplatedCredentials(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, false, phase)
	}

	phase = dbPhaseFinish
	r.Recorder.Event(dbcr, "Normal", phase, "Finishing reconciliation process")
	dbcr.Status.Status = true
	phase = dbPhaseReady
	r.Recorder.Event(dbcr, "Normal", phase, "Ready")

	err = r.Status().Update(ctx, dbcr)
	if err != nil {
		log.Error(err, "error status subresource updating")
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	return reconcileResult, nil
}

func (r *DatabaseReconciler) handleDbDelete(ctx context.Context, dbcr *kindav1beta2.Database) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	var phase string = dbPhaseDelete
	r.Recorder.Event(dbcr, "Normal", phase, "Deleting database")
	reconcilePeriod := r.Interval * time.Second
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Run finalization logic for database. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	if commonhelper.SliceContainsSubString(dbcr.ObjectMeta.Finalizers, "dbuser.") {
		err := errors.New("database can't be removed, while there are DbUser referencing it")
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	if commonhelper.ContainsString(dbcr.ObjectMeta.Finalizers, "db."+dbcr.Name) {
		err := r.deleteDatabase(ctx, dbcr)
		if err != nil {
			log.Error(err, "failed deleting database")
			// when database deletion failed, don't requeue request. to prevent exceeding api limit (ex: against google api)
			return r.manageError(ctx, dbcr, err, false, phase)
		}
		kci.RemoveFinalizer(&dbcr.ObjectMeta, "db."+dbcr.Name)
		err = r.Update(ctx, dbcr)
		if err != nil {
			log.Error(err, "error resource updating")
			return r.manageError(ctx, dbcr, err, true, phase)
		}
	}
	if err := r.handleTemplatedCredentials(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, false, phase)
	}

	if err := r.handleInfoConfigMap(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	dbSecret, err := r.getDatabaseSecret(ctx, dbcr)

	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return r.manageError(ctx, dbcr, err, true, phase)
		}
	} else {
		if err := r.kubeHelper.ModifyObject(ctx, dbSecret); err != nil {
			return r.manageError(ctx, dbcr, err, true, phase)
		}
	}

	return reconcileResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	eventFilter := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isWatchedNamespace(r.WatchNamespaces, e.Object) && isDatabase(e.Object)
		}, // Reconcile only Database Create Event
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isWatchedNamespace(r.WatchNamespaces, e.Object) && isDatabase(e.Object)
		}, // Reconcile only Database Delete Event
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isWatchedNamespace(r.WatchNamespaces, e.ObjectNew) && isObjectUpdated(e)
		}, // Reconcile Database and Secret Update Events
		GenericFunc: func(e event.GenericEvent) bool { return true }, // Reconcile any Generic Events (operator POD or cluster restarted)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindav1beta2.Database{}).
		WithEventFilter(eventFilter).
		Watches(&corev1.Secret{}, &secretEventHandler{r.Client}).
		Complete(r)
}

func (r *DatabaseReconciler) setEngine(ctx context.Context, dbcr *kindav1beta2.Database) error {
	log := log.FromContext(ctx)
	if len(dbcr.Spec.Instance) == 0 {
		return errors.New("instance name not defined")
	}

	if len(dbcr.Status.Engine) == 0 {
		instance := &kindav1beta2.DbInstance{}
		key := types.NamespacedName{
			Namespace: "",
			Name:      dbcr.Spec.Instance,
		}
		err := r.Get(ctx, key, instance)
		if err != nil {
			log.Error(err, "couldn't get instance")
			return err
		}

		if !instance.Status.Connected {
			return errors.New("db-instance connection is not successful")
		}

		dbcr.Status.Engine = instance.Spec.Engine
		if err := r.Status().Update(ctx, dbcr); err != nil {
			return err
		}
		return nil
	}

	return nil
}

// createDatabase secret, actual database using admin secret
func (r *DatabaseReconciler) createDatabase(ctx context.Context, dbcr *kindav1beta2.Database, dbSecret *corev1.Secret) error {
	log := log.FromContext(ctx)
	databaseCred, err := dbhelper.ParseDatabaseSecretData(dbcr, dbSecret.Data)
	if err != nil {
		// failed to parse database credential from secret
		return err
	}
	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}
	dbuser.AccessType = database.ACCESS_TYPE_MAINUSER

	adminCred, err := r.getAdminUser(ctx, dbcr)
	if err != nil {
		return err
	}

	err = database.CreateDatabase(ctx, db, adminCred)
	if err != nil {
		return err
	}

	err = database.CreateOrUpdateUser(ctx, db, dbuser, adminCred)
	if err != nil {
		return err
	}

	kci.AddFinalizer(&dbcr.ObjectMeta, "db."+dbcr.Name)

	commonhelper.AddDBChecksum(dbcr, dbSecret)
	err = r.Update(ctx, dbcr)
	if err != nil {
		return err
	}

	err = r.Update(ctx, dbcr)
	if err != nil {
		log.Error(err, "error resource updating")
		return err
	}

	dbcr.Status.OperatorVersion = commonhelper.OperatorVersion
	dbcr.Status.DatabaseName = databaseCred.Name
	dbcr.Status.UserName = databaseCred.Username
	log.Info("successfully created")
	return nil
}

func (r *DatabaseReconciler) deleteDatabase(ctx context.Context, dbcr *kindav1beta2.Database) error {
	log := log.FromContext(ctx)
	if dbcr.Spec.DeletionProtected {
		log.Info("database is deletion protected, it will not be deleted in backends")
		return nil
	}

	databaseCred := database.Credentials{
		Name:     dbcr.Status.DatabaseName,
		Username: dbcr.Status.UserName,
	}

	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}
	dbuser.AccessType = database.ACCESS_TYPE_MAINUSER

	adminCred, err := r.getAdminUser(ctx, dbcr)
	if err != nil {
		return err
	}

	err = database.DeleteDatabase(ctx, db, adminCred)
	if err != nil {
		return err
	}

	err = database.DeleteUser(ctx, db, dbuser, adminCred)
	if err != nil {
		return err
	}

	return nil
}

// If database has a deletion timestamp, this function will remove all the templated fields from
// secrets and configmaps, so it's a generic function that can be used for both:
// creating and removing
func (r *DatabaseReconciler) handleTemplatedCredentials(ctx context.Context, dbcr *kindav1beta2.Database) error {
	databaseSecret, err := r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		return err
	}

	databaseConfigMap, err := r.getDatabaseConfigMap(ctx, dbcr)
	if err != nil {
		return err
	}

	creds, err := dbhelper.ParseDatabaseSecretData(dbcr, databaseSecret.Data)
	if err != nil {
		return err
	}

	// We don't need dbuser here, because if it's not nil, templates will be built for the dbuser, not the database
	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, creds, instance)
	if err != nil {
		return err
	}

	templateds, err := templates.NewTemplateDataSource(dbcr, nil, databaseSecret, databaseConfigMap, db, dbuser)
	if err != nil {
		return err
	}

	if !dbcr.IsDeleted() {
		if err := templateds.Render(dbcr.Spec.Credentials.Templates); err != nil {
			return err
		}
	} else {
		// Render with an empty slice, so tempalted entries are removed from Data and Annotations
		if err := templateds.Render(kindav1beta2.Templates{}); err != nil {
			return err
		}
	}

	if err := r.kubeHelper.HandleCreateOrUpdate(ctx, templateds.SecretK8sObj); err != nil {
		return err
	}

	if err := r.kubeHelper.HandleCreateOrUpdate(ctx, templateds.ConfigMapK8sObj); err != nil {
		return err
	}
	// Set it to nil explicitly to ensure it's picked up by the GC
	templateds = nil
	return nil
}

func (r *DatabaseReconciler) handleInfoConfigMap(ctx context.Context, dbcr *kindav1beta2.Database) error {
	log := log.FromContext(ctx)
	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	info := map[string]string{
		"DB_CONN": instance.Status.URL,
		"DB_PORT": strconv.FormatInt(instance.Status.Port, 10),
	}

	sslMode, err := dbhelper.GetSSLMode(dbcr, instance)
	if err != nil {
		return err
	}
	info["SSL_MODE"] = sslMode
	// TODO: Remove configmap builder
	databaseConfigResource := kci.ConfigMapBuilder(dbcr.Spec.Credentials.SecretName, dbcr.Namespace, info)

	if err := r.kubeHelper.ModifyObject(ctx, databaseConfigResource); err != nil {
		return err
	}

	log.Info("database info configmap created")
	return nil
}

func (r *DatabaseReconciler) getDatabaseSecret(ctx context.Context, dbcr *kindav1beta2.Database) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: dbcr.Namespace,
		Name:      dbcr.Spec.Credentials.SecretName,
	}
	err := r.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *DatabaseReconciler) getDatabaseConfigMap(ctx context.Context, dbcr *kindav1beta2.Database) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: dbcr.Namespace,
		Name:      dbcr.Spec.Credentials.SecretName,
	}
	err := r.Get(ctx, key, configMap)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (r *DatabaseReconciler) getAdminUser(ctx context.Context, dbcr *kindav1beta2.Database) (*database.DatabaseUser, error) {
	instance := &kindav1beta2.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return nil, err
	}

	// get database admin credentials
	from := instance.Spec.AdminCredentials.UsernameFrom
	username, err := r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
	if err != nil {
		return nil, err
	}

	from = instance.Spec.AdminCredentials.PasswordFrom
	password, err := r.kubeHelper.GetValueFrom(ctx, from.Kind, from.Namespace, from.Name, from.Key)
	if err != nil {
		return nil, err
	}

	dbuser := &database.DatabaseUser{
		Username: username,
		Password: password,
	}

	return dbuser, nil
}

func (r *DatabaseReconciler) manageError(ctx context.Context, dbcr *kindav1beta2.Database, issue error, requeue bool, phase string) (reconcile.Result, error) {
	dbcr.Status.Status = false
	log := log.FromContext(ctx)
	log.Error(issue, "an error occurred during the reconciliation")
	promDBsPhaseError.WithLabelValues(phase).Inc()

	retryInterval := 60 * time.Second

	r.Recorder.Event(dbcr, "Warning", "Failed"+phase, issue.Error())
	err := r.Status().Update(ctx, dbcr)
	if err != nil {
		log.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: retryInterval,
			Requeue:      requeue,
		}, nil
	}

	// TODO: implementing reschedule calculation based on last updated time
	return reconcile.Result{
		RequeueAfter: retryInterval,
		Requeue:      requeue,
	}, nil
}

func (r *DatabaseReconciler) createSecret(ctx context.Context, dbcr *kindav1beta2.Database) (*corev1.Secret, error) {
	log := log.FromContext(ctx)
	secretData, err := dbhelper.GenerateDatabaseSecretData(dbcr.ObjectMeta, string(dbcr.Status.Engine), "")
	if err != nil {
		log.Error(err, "can not generate credentials for database")
		return nil, err
	}

	databaseSecret := kci.SecretBuilder(dbcr.Spec.Credentials.SecretName, dbcr.Namespace, secretData)
	return databaseSecret, nil
}
