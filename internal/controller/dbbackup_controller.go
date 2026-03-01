/*
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

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	commonhelper "github.com/db-operator/db-operator/v2/internal/helpers/common"
	dbhelper "github.com/db-operator/db-operator/v2/internal/helpers/database"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kindarocksv1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/helpers/database"
	kubehelper "github.com/db-operator/db-operator/v2/internal/helpers/kube"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// DbBackupReconciler reconciles a DbBackup object
type DbBackupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Opts     *DbBackupReconcilerOpts
	Recorder events.EventRecorder
}

// Options for the DbBackupReconciler
type DbBackupReconcilerOpts struct {
	// A path to a directory with manifests templates
	kubeHelper             *kubehelper.KubeHelper
	resourceHolderName     string
	resourceHolderID       string
	childObjName           string
	dbCredsSecretName      string
	storageCredsSecretName string
	Namespace              string
	ReconcileAfter         time.Duration
}

// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;patch;get;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=create;patch;get;delete
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// This controller must ensure that state conflicts are not happening, because the external pod
// will fail in case this operator is updating the object that must be consumed by the
// backup tool
func (r *DbBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Prepare required resources
	var err error
	log := logf.FromContext(ctx)
	log.Info("Started a reconciliation")

	// Get the DbBackup from the cluster
	dbbackupcr := &kindarocksv1beta1.DbBackup{}
	if err = r.Get(ctx, req.NamespacedName, dbbackupcr); err != nil {
		if k8serrors.IsNotFound(err) {
			// Object wasn't found, probably it was removed
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get DbBackup")
		return ctrl.Result{}, err
	}

	// Since owner references must be set to resources in the same namespace,
	// we are creating an additional ConfigMap that will be a holder
	// for all resources required for a backup, to make cleanup easier
	immutableResHolder := true
	resourceHolder := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dbbackupcr.Name + "backup-holder",
			Namespace:   dbbackupcr.Namespace,
			Labels:      dbbackupcr.Labels,
			Annotations: dbbackupcr.Annotations,
		},
		Immutable: &immutableResHolder,
		Data: map[string]string{
			"Descrpiption": "This resource is needed to set owner references",
		},
	}

	// If status conditions are not set, it probably means that the CR was just created.
	// Set the status conditions and return to let the next reconcile loop continue the logic
	log.Info("Checking if DbBackup is initialized")
	if len(dbbackupcr.Status.Conditions) == 0 {
		if err := r.initDbBackupCR(ctx, dbbackupcr); err != nil {
			log.Error(err, "Couldn't initialize a DbBackup object")
			return ctrl.Result{}, err
		}
		// Return here, because the status change triggers a new reconciliation
		return ctrl.Result{}, nil
	}

	// If object is removed, run the cleanup
	log.Info("Checking if DbBackup is deleted")
	if dbbackupcr.IsDeleted() {
		log.Info("Resource is deleted, cleaning up")
		if err := r.cleanup(ctx, dbbackupcr, resourceHolder); err != nil {
			log.Error(err, "Couldn't execute the cleanup logic")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// If success, we don't need to do anything
	log.Info("Checking if DbBackup is succeded")
	if r.isSuccess(dbbackupcr) {
		if err := r.cleanup(ctx, dbbackupcr, resourceHolder); err != nil {
			log.Error(err, "Couldn't execute the cleanup logic")
			return ctrl.Result{}, err
		}
		status := true
		dbbackupcr.Status.Status = &status
		if err := r.Status().Update(ctx, dbbackupcr); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Backup is already processed successfully")
		return ctrl.Result{}, nil
	}

	// If object status is false, do not retry
	log.Info("Checking if DbBackup is failed")
	if r.isFailed(dbbackupcr) {
		log.Info("The object status is false, not retrying")
		return ctrl.Result{}, nil
	}

	running, err := r.isBackupRunning(ctx, dbbackupcr)
	if err != nil {
		log.Error(err, "Couldn't get the pod status, continuing ...")
	}

	if running {
		log.Info("Backup pod is running, waiting ...")
		return ctrl.Result{}, nil
	}

	// The backup pod is using this field to lock the object,
	// Operator must wait until the object is unlocked
	log.Info("Checking if DbBackup is locked by a backup pod")
	if *dbbackupcr.Status.LockedByBackupJob {
		log.Info("Locked by a backup job, skipping ...")
		return ctrl.Result{}, nil
	}

	r.Opts.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbbackupcr)

	log.Info("Checking is resource holder is ready")
	// Create a resource holder and add a finalizer to DbBackup
	if !meta.IsStatusConditionTrue(dbbackupcr.Status.Conditions, consts.TYPE_RESOURCE_HOLDER) {
		r.Recorder.Eventf(dbbackupcr, resourceHolder, corev1.EventTypeNormal, "DbBackup", "ResourceHolder", "Creating a resource holder")
		_, err := r.Opts.kubeHelper.Create(ctx, resourceHolder)
		if err != nil {
			log.Error(err, "Couldn't create a resource hodler", "name", resourceHolder.Name)
			return ctrl.Result{}, err
		}

		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_RESOURCE_HOLDER, Status: metav1.ConditionTrue, Reason: "Created", Message: "Resource holder is ready"},
		)

		if err := r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Couldn't update status")
			return ctrl.Result{}, err
		}

		// Return to avoid additional conflicts
		return ctrl.Result{}, nil
	}

	log.Info("Checking if resource holder finalizer is added")
	// Set the finalizer if needed
	if meta.IsStatusConditionTrue(dbbackupcr.Status.Conditions, consts.TYPE_RESOURCE_HOLDER) {
		if !slices.Contains(dbbackupcr.Finalizers, consts.FIN_RESOURCE_HOLDER) {
			r.Recorder.Eventf(dbbackupcr, resourceHolder, corev1.EventTypeNormal, "DbBackup", "ResourceHolder", "Adding a resource holder finalizer")
			dbbackupcr.Finalizers = append(dbbackupcr.Finalizers, consts.FIN_RESOURCE_HOLDER)

			if err := r.Update(ctx, dbbackupcr); err != nil {
				log.Error(err, "Couldn't add a finalizer")
				return ctrl.Result{}, err
			}

			// Return to avoid additional conflicts
			return ctrl.Result{}, nil
		}
	}

	log.Info("Getting a new resource holder from Kubernetes")
	// Get the new verson of a resource holder
	if err := r.Get(ctx, types.NamespacedName{Namespace: resourceHolder.Namespace, Name: resourceHolder.Name}, resourceHolder); err != nil {
		log.Error(err, "Couldn't get a resource hodler")
		return ctrl.Result{}, err
	}

	r.Opts.resourceHolderID = string(resourceHolder.UID)
	r.Opts.resourceHolderName = resourceHolder.Name
	r.Opts.childObjName = fmt.Sprintf("%s-backup", dbbackupcr.Name)
	r.Opts.dbCredsSecretName = fmt.Sprintf("%s-db", r.Opts.childObjName)
	r.Opts.storageCredsSecretName = fmt.Sprintf("%s-storage", r.Opts.childObjName)

	log.Info("Getting a database for this backup")
	// Try to get the database for a backup
	dbcr := &kindarocksv1beta1.Database{}
	if err = r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: *dbbackupcr.Spec.Database}, dbcr); err != nil {
		// If we can't get a database, doesn't make sense to continue
		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_BACKUP_STATUS, Status: metav1.ConditionFalse, Reason: "Failed", Message: "Database doesn't exist"},
		)

		if err = r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Failed to update DbBackup status")
			return ctrl.Result{}, err
		}

		log.Error(err, "Database can't be found", "database", *dbbackupcr.Spec.Database)
		return ctrl.Result{}, err
	}

	log.Info("Checking if database is ready")
	// If database is not ready, we need to try later
	if !dbcr.Status.Status {
		err := errors.New("database is not ready")
		log.Error(err, "Database is not ready", "namespace", dbbackupcr.Namespace, "database", *dbbackupcr.Spec.Database)
		return ctrl.Result{Requeue: true}, err
	}

	log.Info("Setting the database engine")
	// Set the engine
	if dbbackupcr.Status.Engine == nil {
		r.Recorder.Eventf(dbbackupcr, dbcr, corev1.EventTypeNormal, "DbBackup", "Engine", "Setting the engine")
		dbbackupcr.Status.Engine = &dbcr.Status.Engine
		if err := r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Couldn't set database engine in the status")
			return ctrl.Result{}, err
		}
		// Return here, because the status change triggers a new reconciliation
		return ctrl.Result{}, nil
	}

	log.Info("Checking if retry limit is reached")
	// If not backup.success, create a backup pod
	// Check if db-operator has reached the possible amount of retries
	// If yes, give up and set status to false
	if *dbbackupcr.Status.FailedRetries >= *dbbackupcr.Spec.Retries {
		r.Recorder.Eventf(dbbackupcr, dbcr, corev1.EventTypeWarning, "DbBackup", "Failed", "Backup was failed")
		err := errors.New("failed retries amount is reached")
		log.Error(err, "The amount of  failed retries is reached, CR is marked as failed", "retry", dbbackupcr.Status.FailedRetries)

		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_BACKUP_STATUS, Status: metav1.ConditionFalse, Reason: "Failed", Message: "Reached the amount of possible failed retries"},
		)

		if err = r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Failed to update DbBackup status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: r.Opts.ReconcileAfter}, nil
	}

	log.Info("Executing the backup logic", "retry", *dbbackupcr.Status.FailedRetries+1)
	// Init the kubehelper object
	// Create a service account
	// Create a DB Secret
	// Create a POD

	log.Info("Creating a Service Account")
	if err := r.createSA(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a service account")
		return ctrl.Result{}, err
	}

	log.Info("Creating a database Secret")
	if err := r.createDbSecret(ctx, dbbackupcr, dbcr); err != nil {
		log.Error(err, "Couldn't create a secret with database credentials")
		return ctrl.Result{}, err
	}

	log.Info("Creating a storage Secret")
	if err := r.createUploadSecret(ctx, dbbackupcr, dbcr); err != nil {
		log.Error(err, "Couldn't create a secret with storage credentials")
		return ctrl.Result{}, err
	}

	log.Info("Creating a Role")
	if err := r.createRole(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a role")
		return ctrl.Result{}, err
	}

	log.Info("Creating a Role Binding")
	if err := r.createRoleBinding(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a role binding")
		return ctrl.Result{}, err
	}

	log.Info("Creating a Pod")
	if err := r.createPod(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a pod")
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(ctx, dbbackupcr); err != nil {
		log.Error(err, "Coudln't update DbBackup status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DbBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindarocksv1beta1.DbBackup{}).
		Named("dbbackup").
		Complete(r)
}

// Set the initial condition to the DbBackup and updates the status
func (r *DbBackupReconciler) initDbBackupCR(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	log := logf.FromContext(ctx)
	log.Info("Initializing an object")
	r.Recorder.Eventf(obj, nil, corev1.EventTypeNormal, "DbBackup", "Initialization", "Initializing the DbBackup object")
	obj.Status.OperatorVersion = &commonhelper.OperatorVersion
	meta.SetStatusCondition(
		&obj.Status.Conditions,
		metav1.Condition{Type: consts.TYPE_BACKUP_STATUS, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
	)
	meta.SetStatusCondition(
		&obj.Status.Conditions,
		metav1.Condition{Type: consts.TYPE_RESOURCE_HOLDER, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
	)
	if err := r.Status().Update(ctx, obj); err != nil {
		log.V(2).Info("Failed to update DbBackup status", "error", err)
		return err
	}
	return nil
}

// Check if DbBackup is already failed
func (r *DbBackupReconciler) isFailed(obj *kindarocksv1beta1.DbBackup) bool {
	return meta.IsStatusConditionFalse(obj.Status.Conditions, consts.TYPE_BACKUP_STATUS)
}

// Check if DbBackup is already succeded
func (r *DbBackupReconciler) isSuccess(obj *kindarocksv1beta1.DbBackup) bool {
	return meta.IsStatusConditionTrue(obj.Status.Conditions, consts.TYPE_BACKUP_STATUS)
}

func (r *DbBackupReconciler) isBackupRunning(ctx context.Context, obj *kindarocksv1beta1.DbBackup) (bool, error) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: *obj.Status.BackupPodName}, pod); err != nil {
		return false, err
	}
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
		return true, nil
	}
	return false, nil
}

// Remove the resource holder and a finalizer from the DbBackup
func (r *DbBackupReconciler) cleanup(ctx context.Context, obj *kindarocksv1beta1.DbBackup, cm *corev1.ConfigMap) error {
	log := logf.FromContext(ctx)
	r.Recorder.Eventf(obj, nil, corev1.EventTypeNormal, "DbBackup", "CleanUp", "Cleaning up after a backup")
	if *obj.Spec.Debug {
		log.Info("Debug mod is turned on, not removing anything")
		return nil
	}
	log.Info("Executing the cleanup logic")
	if err := r.Delete(ctx, cm); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}
	obj.Finalizers = slices.DeleteFunc(obj.Finalizers, func(item string) bool {
		return item == consts.FIN_RESOURCE_HOLDER
	})

	if err := r.Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func (r *DbBackupReconciler) createSA(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Service Account")
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.childObjName,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
	}
	r.Recorder.Eventf(obj, sa, corev1.EventTypeNormal, "DbBackup", "ServiceAccount", "Creating a service account")
	if err := r.Create(ctx, sa); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (r *DbBackupReconciler) createRole(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Role")
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.childObjName,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{"kinda.rocks"},
				Resources:     []string{"dbbackups"},
				ResourceNames: []string{obj.Name},
			},
			{
				Verbs:         []string{"get", "patch", "update"},
				APIGroups:     []string{"kinda.rocks"},
				Resources:     []string{"dbbackups/status"},
				ResourceNames: []string{obj.Name},
			},
		},
	}
	r.Recorder.Eventf(obj, role, corev1.EventTypeNormal, "DbBackup", "Role", "Creating a role")
	if err := r.Create(ctx, role); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (r *DbBackupReconciler) createRoleBinding(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Role Binding")
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.childObjName,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			APIGroup:  "",
			Name:      r.Opts.childObjName,
			Namespace: obj.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     r.Opts.childObjName,
		},
	}
	r.Recorder.Eventf(obj, roleBinding, corev1.EventTypeNormal, "DbBackup", "RoleBinding", "Creating a role binding")
	if err := r.Create(ctx, roleBinding); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (r *DbBackupReconciler) createDbSecret(ctx context.Context, obj *kindarocksv1beta1.DbBackup, dbcr *kindarocksv1beta1.Database) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Secret with DB credentials")
	immutable := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.dbCredsSecretName,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
		Immutable: &immutable,
		Data:      map[string][]byte{},
	}

	// Getting credentials for performing a backup
	dbSecret, err := r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		return err
	}

	databaseCred, err := dbhelper.ParseDatabaseSecretData(dbcr, dbSecret.Data)
	if err != nil {
		return err
	}

	instance := &kindarocksv1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	db, _, err := database.FetchDatabaseData(ctx, dbcr, databaseCred, instance)

	envData := map[string][]byte{}
	envData["DB_HOST"] = []byte(db.GetDatabaseAddress(ctx).Host)
	envData["DB_PORT"] = []byte(strconv.FormatUint(uint64(db.GetDatabaseAddress(ctx).Port), 10))
	envData["DB_DATABASE"] = []byte(databaseCred.DatabaseName)
	envData["DB_PASSWORD"] = []byte(databaseCred.Password)
	envData["DB_USER"] = []byte(databaseCred.Username)
	secret.Data = envData

	r.Recorder.Eventf(obj, secret, corev1.EventTypeNormal, "DbBackup", "DbSecret", "Creating a secret with database credentials")
	if err := r.Create(ctx, secret); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (r *DbBackupReconciler) createUploadSecret(ctx context.Context, obj *kindarocksv1beta1.DbBackup, dbcr *kindarocksv1beta1.Database) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Secret with external storage credentials")
	immutable := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.storageCredsSecretName,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
		Immutable: &immutable,
		Data:      map[string][]byte{},
	}

	instance := &kindarocksv1beta1.DbInstance{}
	if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
		return err
	}

	value, ok := instance.Spec.Backup.AvailableSecrets[*obj.Spec.StorageCredentials]
	if !ok {
		return errors.New("upload credentials secret is not found")
	}

	credSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: value.Namespace, Name: value.Name}, credSecret); err != nil {
		return err
	}

	secret.Data = credSecret.Data

	r.Recorder.Eventf(obj, secret, corev1.EventTypeNormal, "DbBackup", "StorageSecret", "Creating a secret with storage credentials")
	if err := r.Create(ctx, secret); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (r *DbBackupReconciler) createPod(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	log := logf.FromContext(ctx)
	log.Info("Creating a Pod")
	containerRestartPolicy := corev1.ContainerRestartPolicyNever
	podRestartPolicy := corev1.RestartPolicyOnFailure

	retry := strconv.FormatInt(int64(*obj.Status.FailedRetries), 10)

	image := fmt.Sprintf("%s/%s:%s", *obj.Spec.Image.Registry, *obj.Spec.Image.Repository, *obj.Spec.Image.Tag)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.Opts.childObjName + "-" + retry,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       r.Opts.resourceHolderName,
				UID:        types.UID(r.Opts.resourceHolderID),
			}},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: podRestartPolicy,
			Volumes: []corev1.Volume{{
				Name: "backup",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
			InitContainers: []corev1.Container{
				{
					RestartPolicy: &containerRestartPolicy,
					Name:          "discover",
					Image:         image,
					Args: []string{
						"backup",
						"discover",
						"--namespace",
						obj.Namespace,
						"--backup-name",
						obj.Name,
					},
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.Opts.storageCredsSecretName,
							},
						},
					}},
					Env: []corev1.EnvVar{
						{
							Name:  "DB_BACKUP_NAME",
							Value: obj.Name,
						},
						{
							Name:  "DB_BACKUP_NAMESPACE",
							Value: obj.Namespace,
						},
					},
					ImagePullPolicy: corev1.PullPolicy(*obj.Spec.Image.PullPolicy),
				},
				{
					RestartPolicy: &containerRestartPolicy,
					Name:          "backup",
					Image:         image,
					Args: []string{
						"backup",
						"dump",
						"--namespace",
						obj.Namespace,
						"--backup-name",
						obj.Name,
					},
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.Opts.dbCredsSecretName,
							},
						},
					}},
					Env: []corev1.EnvVar{
						{
							Name:  "DB_BACKUP_NAME",
							Value: obj.Name,
						},
						{
							Name:  "DB_BACKUP_NAMESPACE",
							Value: obj.Namespace,
						},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "backup",
						MountPath: "/backup",
					}},
					ImagePullPolicy: corev1.PullPolicy(*obj.Spec.Image.PullPolicy),
				},
				{
					RestartPolicy: &containerRestartPolicy,
					Name:          "upload",
					Image:         image,
					Args: []string{
						"backup",
						"upload",
						"--namespace",
						obj.Namespace,
						"--backup-name",
						obj.Name,
					},
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.Opts.storageCredsSecretName,
							},
						},
					}},
					Env: []corev1.EnvVar{
						{
							Name:  "DB_BACKUP_NAME",
							Value: obj.Name,
						},
						{
							Name:  "DB_BACKUP_NAMESPACE",
							Value: obj.Namespace,
						},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "backup",
						MountPath: "/backup",
					}},
					ImagePullPolicy: corev1.PullPolicy(*obj.Spec.Image.PullPolicy),
				},
			},
			Containers: []corev1.Container{
				{
					RestartPolicy: &containerRestartPolicy,
					Name:          "describe",
					Image:         image,
					Args: []string{
						"backup",
						"describe",
						"--namespace",
						obj.Namespace,
						"--backup-name",
						obj.Name,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "DB_BACKUP_NAME",
							Value: obj.Name,
						},
						{
							Name:  "DB_BACKUP_NAMESPACE",
							Value: obj.Namespace,
						},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "backup",
						MountPath: "/backup",
					}},
					ImagePullPolicy: corev1.PullPolicy(*obj.Spec.Image.PullPolicy),
				},
			},
			ServiceAccountName: r.Opts.childObjName,
		},
	}

	r.Recorder.Eventf(obj, pod, corev1.EventTypeNormal, "DbBackup", "Pod", "Creating a backup pod")
	if err := r.Create(ctx, pod); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	obj.Status.BackupPodName = &pod.Name
	return nil
}

// TODO: It needs not to be copy-pasted for each controller
func (r *DbBackupReconciler) getDatabaseSecret(ctx context.Context, dbcr *kindarocksv1beta1.Database) (*corev1.Secret, error) {
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
