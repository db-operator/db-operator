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
	"os"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kindav1beta1 "github.com/db-operator/db-operator/api/v1beta1"
	"github.com/db-operator/db-operator/internal/controller/backup"
	commonhelper "github.com/db-operator/db-operator/internal/helpers/common"
	dbhelper "github.com/db-operator/db-operator/internal/helpers/database"
	kubehelper "github.com/db-operator/db-operator/internal/helpers/kube"
	proxyhelper "github.com/db-operator/db-operator/internal/helpers/proxy"
	"github.com/db-operator/db-operator/internal/utils/templates"
	"github.com/db-operator/db-operator/pkg/config"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/db-operator/db-operator/pkg/utils/database"
	"github.com/db-operator/db-operator/pkg/utils/kci"
	"github.com/db-operator/db-operator/pkg/utils/proxy"
	secTemplates "github.com/db-operator/db-operator/pkg/utils/templates"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	dbPhaseReconcile            = "Reconciling"
	dbPhaseCreateOrUpdate       = "CreatingOrUpdating"
	dbPhaseInstanceAccessSecret = "InstanceAccessSecretCreating"
	dbPhaseProxy                = "ProxyCreating"
	dbPhaseSecretsTemplating    = "SecretsTemplating"
	dbPhaseConfigMap            = "InfoConfigMapCreating"
	dbPhaseTemplating           = "Templating"
	dbPhaseBackupJob            = "BackupJobCreating"
	dbPhaseFinish               = "Finishing"
	dbPhaseReady                = "Ready"
	dbPhaseDelete               = "Deleting"
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
	dbcr := &kindav1beta1.Database{}
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
func (r *DatabaseReconciler) healthCheck(ctx context.Context, dbcr *kindav1beta1.Database) error {
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

	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}

	if err := db.CheckStatus(dbuser); err != nil {
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
func (r *DatabaseReconciler) isFullReconcile(ctx context.Context, dbcr *kindav1beta1.Database) (bool, error) {
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
func (r *DatabaseReconciler) handleDbCreateOrUpdate(ctx context.Context, dbcr *kindav1beta1.Database, mustReconcile bool) (reconcile.Result, error) {
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

	phase = dbPhaseInstanceAccessSecret
	r.Recorder.Event(dbcr, "Normal", phase, "Creating secret containing credentials to access the database instance")
	if err := r.handleInstanceAccessSecret(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}
	log.Info("instance access secret created")

	phase = dbPhaseProxy
	r.Recorder.Event(dbcr, "Normal", phase, "Creating proxy")
	err = r.handleProxy(ctx, dbcr)
	if err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	phase = dbPhaseSecretsTemplating
	r.Recorder.Event(dbcr, "Normal", phase, "Creating templated secrets")
	if err = r.createTemplatedSecrets(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}
	phase = dbPhaseConfigMap
	r.Recorder.Event(dbcr, "Normal", phase, "Creating ConfigMap")
	if err = r.handleInfoConfigMap(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}
	phase = dbPhaseTemplating
	r.Recorder.Event(dbcr, "Normal", phase, "Handle templated credentials")

	// A temporary check that exists to avoid creating templates if secretsTemplates are used.
	// todo: It should be removed when secretsTemlates are gone

	if len(dbcr.Spec.SecretsTemplates) == 0 {
		if err := r.handleTemplatedCredentials(ctx, dbcr); err != nil {
			return r.manageError(ctx, dbcr, err, false, phase)
		}
	}
	phase = dbPhaseBackupJob
	r.Recorder.Event(dbcr, "Normal", phase, "Handle BackupJob")
	err = r.handleBackupJob(ctx, dbcr)
	if err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
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

func (r *DatabaseReconciler) handleDbDelete(ctx context.Context, dbcr *kindav1beta1.Database) (reconcile.Result, error) {
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
	// A temporary check that exists to avoid creating templates if secretsTemplates are used.
	// todo: It should be removed when secretsTemlates are gone
	if len(dbcr.Spec.SecretsTemplates) == 0 {
		if err := r.handleTemplatedCredentials(ctx, dbcr); err != nil {
			return r.manageError(ctx, dbcr, err, false, phase)
		}
	}

	if err := r.handleInstanceAccessSecret(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	if err := r.handleProxy(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	if err := r.handleInfoConfigMap(ctx, dbcr); err != nil {
		return r.manageError(ctx, dbcr, err, true, phase)
	}

	if err := r.handleBackupJob(ctx, dbcr); err != nil {
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
		For(&kindav1beta1.Database{}).
		WithEventFilter(eventFilter).
		Watches(&corev1.Secret{}, &secretEventHandler{r.Client}).
		Complete(r)
}

func (r *DatabaseReconciler) setEngine(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
	if len(dbcr.Spec.Instance) == 0 {
		return errors.New("instance name not defined")
	}

	if len(dbcr.Status.Engine) == 0 {
		instance := &kindav1beta1.DbInstance{}
		key := types.NamespacedName{
			Namespace: "",
			Name:      dbcr.Spec.Instance,
		}
		err := r.Get(ctx, key, instance)
		if err != nil {
			log.Error(err, "couldn't get instance")
			return err
		}

		if !instance.Status.Status {
			return errors.New("instance status not true")
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
func (r *DatabaseReconciler) createDatabase(ctx context.Context, dbcr *kindav1beta1.Database, dbSecret *corev1.Secret) error {
	log := log.FromContext(ctx)
	databaseCred, err := dbhelper.ParseDatabaseSecretData(dbcr, dbSecret.Data)
	if err != nil {
		// failed to parse database credential from secret
		return err
	}
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}
	dbuser.AccessType = database.ACCESS_TYPE_MAINUSER

	adminSecretResource, err := r.getAdminSecret(ctx, dbcr)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Error(err, "can not find admin secret")
			return err
		}
		return err
	}

	// found admin secret. parse it to connect database
	adminCred, err := db.ParseAdminCredentials(adminSecretResource.Data)
	if err != nil {
		// failed to parse database admin secret
		return err
	}

	err = database.CreateDatabase(db, adminCred)
	if err != nil {
		return err
	}

	err = database.CreateOrUpdateUser(db, dbuser, adminCred)
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

	dbcr.Status.DatabaseName = databaseCred.Name
	dbcr.Status.UserName = databaseCred.Username
	log.Info("successfully created")
	return nil
}

func (r *DatabaseReconciler) deleteDatabase(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
	if dbcr.Spec.DeletionProtected {
		log.Info("database is deletion protected, it will not be deleted in backends")
		return nil
	}

	databaseCred := database.Credentials{
		Name:     dbcr.Status.DatabaseName,
		Username: dbcr.Status.UserName,
	}

	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
	if err != nil {
		// failed to determine database type
		return err
	}
	dbuser.AccessType = database.ACCESS_TYPE_MAINUSER

	adminSecretResource, err := r.getAdminSecret(ctx, dbcr)
	if err != nil {
		// failed to get admin secret
		return err
	}
	// found admin secret. parse it to connect database
	adminCred, err := db.ParseAdminCredentials(adminSecretResource.Data)
	if err != nil {
		// failed to parse database admin secret
		return err
	}

	err = database.DeleteDatabase(db, adminCred)
	if err != nil {
		return err
	}

	err = database.DeleteUser(db, dbuser, adminCred)
	if err != nil {
		return err
	}

	return nil
}

func (r *DatabaseReconciler) handleInstanceAccessSecret(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
	var err error
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	if backend, _ := instance.GetBackendType(); backend != "google" {
		log.V(2).Info("doesn't need instance access secret skipping...")
		return nil
	}

	var data []byte

	credFile := "credentials.json"

	if instance.Spec.Google.ClientSecret.Name != "" {
		key := instance.Spec.Google.ClientSecret.ToKubernetesType()
		secret := &corev1.Secret{}
		err := r.Get(ctx, key, secret)
		if err != nil {
			log.Error(err, "can not get instance access secret")
			return err
		}
		data = secret.Data[credFile]
	} else {
		data, err = os.ReadFile(os.Getenv("GCSQL_CLIENT_CREDENTIALS"))
		if err != nil {
			return err
		}
	}
	secretData := make(map[string][]byte)
	secretData[credFile] = data

	newName := dbcr.InstanceAccessSecretName()
	newSecret := kci.SecretBuilder(newName, dbcr.GetNamespace(), secretData)
	if err := r.kubeHelper.ModifyObject(ctx, newSecret); err != nil {
		return err
	}

	return nil
}

func (r *DatabaseReconciler) handleProxy(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	backend, _ := instance.GetBackendType()
	if backend == "generic" {
		log.Info("proxy creation is not yet implemented skipping...")
		return nil
	}

	proxyInterface, err := proxyhelper.DetermineProxyTypeForDB(r.Conf, dbcr, instance)
	if err != nil {
		return err
	}

	// create proxy configmap
	cm, err := proxy.BuildConfigmap(proxyInterface)
	if err != nil {
		return err
	}
	if cm != nil {
		if err := r.kubeHelper.ModifyObject(ctx, cm); err != nil {
			return err
		}
	}

	// create proxy deployment
	deploy, err := proxy.BuildDeployment(proxyInterface)
	if err != nil {
		return err
	}
	if err := r.kubeHelper.ModifyObject(ctx, deploy); err != nil {
		return err
	}

	// create proxy service
	svc, err := proxy.BuildService(proxyInterface)
	if err != nil {
		return err
	}
	if err := r.kubeHelper.ModifyObject(ctx, svc); err != nil {
		return err
	}

	crdList := crdv1.CustomResourceDefinitionList{}
	err = r.List(ctx, &crdList)
	if err != nil {
		return err
	}

	isMonitoringEnabled := instance.IsMonitoringEnabled()
	if isMonitoringEnabled && commonhelper.InCrdList(crdList, "servicemonitors.monitoring.coreos.com") {
		// create proxy PromServiceMonitor
		promSvcMon, err := proxy.BuildServiceMonitor(proxyInterface)
		if err != nil {
			return err
		}
		if err := r.kubeHelper.ModifyObject(ctx, promSvcMon); err != nil {
			return err
		}

	}

	dbcr.Status.ProxyStatus.ServiceName = svc.Name
	for _, svcPort := range svc.Spec.Ports {
		if svcPort.Name == dbcr.Status.Engine {
			dbcr.Status.ProxyStatus.SQLPort = svcPort.Port
		}
	}
	dbcr.Status.ProxyStatus.Status = true

	log.Info("proxy is created")
	return nil
}

// If database has a deletion timestamp, this function will remove all the templated fields from
// secrets and configmaps, so it's a generic function that can be used for both:
// creating and removing
func (r *DatabaseReconciler) handleTemplatedCredentials(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
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
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, dbuser, err := dbhelper.FetchDatabaseData(ctx, dbcr, creds, instance)
	if err != nil {
		return err
	}

	templateds, err := templates.NewTemplateDataSource(dbcr, nil, databaseSecret, databaseConfigMap, db, dbuser, log)
	if err != nil {
		return err
	}

	if !dbcr.IsDeleted() {
		if err := templateds.Render(dbcr.Spec.Credentials.Templates); err != nil {
			return err
		}
	} else {
		// Render with an empty slice, so tempalted entries are removed from Data and Annotations
		if err := templateds.Render(kindav1beta1.Templates{}); err != nil {
			return err
		}
	}

	if err := r.kubeHelper.HandleCreateOrUpdate(ctx, templateds.SecretK8sObj); err != nil {
		return err
	}

	if err := r.kubeHelper.HandleCreateOrUpdate(ctx, templateds.ConfigMapK8sObj); err != nil {
		return err
	}

	return nil
}

func (r *DatabaseReconciler) createTemplatedSecrets(ctx context.Context, dbcr *kindav1beta1.Database) error {
	if len(dbcr.Spec.SecretsTemplates) > 0 {
		r.Recorder.Event(dbcr, "Warning", "Deprecation",
			"secretsTemplates are deprecated and will be removed in the next API version. Please consider using templates",
		)
		// First of all the password should be taken from secret because it's not stored anywhere else
		databaseSecret, err := r.getDatabaseSecret(ctx, dbcr)
		if err != nil {
			return err
		}

		cred, err := dbhelper.ParseDatabaseSecretData(dbcr, databaseSecret.Data)
		if err != nil {
			return err
		}

		databaseCred, err := secTemplates.ParseTemplatedSecretsData(dbcr, cred, databaseSecret.Data)
		if err != nil {
			return err
		}
		instance := &kindav1beta1.DbInstance{}
		if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
			return err
		}

		db, _, err := dbhelper.FetchDatabaseData(ctx, dbcr, databaseCred, instance)
		if err != nil {
			// failed to determine database type
			return err
		}
		dbSecrets, err := secTemplates.GenerateTemplatedSecrets(dbcr, databaseCred, db.GetDatabaseAddress())
		if err != nil {
			return err
		}
		// Adding values
		newSecretData := secTemplates.AppendTemplatedSecretData(dbcr, databaseSecret.Data, dbSecrets)
		newSecretData = secTemplates.RemoveObsoleteSecret(dbcr, newSecretData, dbSecrets)

		for key, value := range newSecretData {
			databaseSecret.Data[key] = value
		}

		if err := r.kubeHelper.ModifyObject(ctx, databaseSecret); err != nil {
			return err
		}

		if err = r.Update(ctx, databaseSecret, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (r *DatabaseReconciler) handleInfoConfigMap(ctx context.Context, dbcr *kindav1beta1.Database) error {
	log := log.FromContext(ctx)
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	info := instance.Status.DeepCopy().Info
	proxyStatus := dbcr.Status.ProxyStatus

	if proxyStatus.Status {
		info["DB_HOST"] = proxyStatus.ServiceName
		info["DB_PORT"] = strconv.FormatInt(int64(proxyStatus.SQLPort), 10)
	}

	sslMode, err := dbhelper.GetSSLMode(dbcr, instance)
	if err != nil {
		return err
	}
	info["SSL_MODE"] = sslMode
	databaseConfigResource := kci.ConfigMapBuilder(dbcr.Spec.SecretName, dbcr.Namespace, info)

	if err := r.kubeHelper.ModifyObject(ctx, databaseConfigResource); err != nil {
		return err
	}

	log.Info("database info configmap created")
	return nil
}

func (r *DatabaseReconciler) handleBackupJob(ctx context.Context, dbcr *kindav1beta1.Database) error {
	if !dbcr.Spec.Backup.Enable {
		// if not enabled, skip
		return nil
	}
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	cronjob, err := backup.GCSBackupCron(r.Conf, dbcr, instance)
	if err != nil {
		return err
	}

	err = controllerutil.SetControllerReference(dbcr, cronjob, r.Scheme)
	if err != nil {
		return err
	}

	if err := r.kubeHelper.ModifyObject(ctx, cronjob); err != nil {
		return err
	}
	return nil
}

func (r *DatabaseReconciler) getDatabaseSecret(ctx context.Context, dbcr *kindav1beta1.Database) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: dbcr.Namespace,
		Name:      dbcr.Spec.SecretName,
	}
	err := r.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *DatabaseReconciler) getDatabaseConfigMap(ctx context.Context, dbcr *kindav1beta1.Database) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: dbcr.Namespace,
		Name:      dbcr.Spec.SecretName,
	}
	err := r.Get(ctx, key, configMap)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (r *DatabaseReconciler) getAdminSecret(ctx context.Context, dbcr *kindav1beta1.Database) (*corev1.Secret, error) {
	instance := &kindav1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return nil, err
	}

	// get database admin credentials
	secret := &corev1.Secret{}

	if err := r.Get(ctx, instance.Spec.AdminUserSecret.ToKubernetesType(), secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *DatabaseReconciler) manageError(ctx context.Context, dbcr *kindav1beta1.Database, issue error, requeue bool, phase string) (reconcile.Result, error) {
	dbcr.Status.Status = false
	log := log.FromContext(ctx)
	dbcr.Status.Status = false
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

func (r *DatabaseReconciler) createSecret(ctx context.Context, dbcr *kindav1beta1.Database) (*corev1.Secret, error) {
	log := log.FromContext(ctx)
	secretData, err := dbhelper.GenerateDatabaseSecretData(dbcr.ObjectMeta, dbcr.Status.Engine, "")
	if err != nil {
		log.Error(err, "can not generate credentials for database")
		return nil, err
	}

	databaseSecret := kci.SecretBuilder(dbcr.Spec.SecretName, dbcr.Namespace, secretData)
	return databaseSecret, nil
}
