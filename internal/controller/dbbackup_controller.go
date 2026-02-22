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

	dbhelper "github.com/db-operator/db-operator/v2/internal/helpers/database"
	batchv1 "k8s.io/api/batch/v1"
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
	kubeHelper *kubehelper.KubeHelper
	Namespace  string
}

const (
	finResourceHolder string = "kinda.rocks/resource-hodler"
)

// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;patch;get;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=create;patch;get;delete
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DbBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Prepare required resources
	var err error
	log := logf.FromContext(ctx)
	log.Info("Started a reconciliation")

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

	if *dbbackupcr.Status.LockedByBackupJob {
		log.Info("Locked by a backup job, skipping ...")
		return ctrl.Result{}, nil
	}

	// If object status is false, do not retry
	if r.isFailed(dbbackupcr) {
		log.Info("The object status is false, not retrying")
		return ctrl.Result{}, nil
	}

	immutableResHolder := true

	// Since owner references must be set to resources in the same namespace,
	// we are creating an additional ConfigMap that will be a holder
	// for all resources required for a backup, to make cleanup easier
	resourceHolder := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dbbackupcr.Name + "holder",
			Namespace:   dbbackupcr.Namespace,
			Labels:      dbbackupcr.Labels,
			Annotations: dbbackupcr.Annotations,
		},
		Immutable: &immutableResHolder,
		Data: map[string]string{
			"Descrpiption": "This resource is needed to set owner references",
		},
	}

	// If object is removed, run the cleanup
	if dbbackupcr.IsDeleted() {
		if err := r.cleanup(ctx, dbbackupcr, resourceHolder); err != nil {
			log.Error(err, "Couldn't execute the cleanup logic")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.isSuccess(dbbackupcr) {
		if err := r.cleanup(ctx, dbbackupcr, resourceHolder); err != nil {
			log.Error(err, "Couldn't execute the cleanup logic")
			return ctrl.Result{}, err
		}
		log.Info("Backup is already processed successfully")
		return ctrl.Result{}, nil
	}

	r.Opts.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbbackupcr)

	// If status conditions are not set, it probably means that the CR was just created.
	// Set the status conditions and return to let the next reconcile loop continue the logic
	if len(dbbackupcr.Status.Conditions) == 0 {
		if err := r.initDbBackupCR(ctx, dbbackupcr); err != nil {
			log.Error(err, "Couldn't initialize a DbBackup object")
			return ctrl.Result{}, err
		}
	}

	// Create a resource holder and add a finalizer to DbBackup
	if !slices.Contains(dbbackupcr.Finalizers, finResourceHolder) {
		_, err := r.Opts.kubeHelper.Create(ctx, resourceHolder)
		if err != nil {
			log.Error(err, "Couldn't create a resource hodler", "name", resourceHolder.Name)
			return ctrl.Result{}, err
		}
		dbbackupcr.Finalizers = append(dbbackupcr.Finalizers, finResourceHolder)
		if err := r.Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Couldn't add a finalizer")
			return ctrl.Result{}, err
		}
	}

	// Try to get the database for a backup
	dbcr := &kindarocksv1beta1.Database{}
	if err = r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: *dbbackupcr.Spec.Database}, dbcr); err != nil {
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

	if !dbcr.Status.Status {
		err := errors.New("database is not ready")
		log.Error(err, "Database is not ready", "namespace", dbbackupcr.Namespace, "database", *dbbackupcr.Spec.Database)
		return ctrl.Result{}, err
	}

	// Prepare required Kubernetes resources: Role, RoleBinding, ServiceAccount, and Pod
	// TODO: Add support for PVC to handle big backups

	// If not backup.success, create a backup pod
	// Check if db-operator has reached the possible amount of retries
	// If yes, give up and set status to false
	if *dbbackupcr.Status.FailedRetries >= *dbbackupcr.Spec.Retries {
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

		return ctrl.Result{}, nil
	}

	log.Info("Executing the backup logic", "retry", *dbbackupcr.Status.FailedRetries+1)

	// Init the kubehelper object
	// Create a service account
	// Create a DB Secret
	// Create a POD

	if err := r.createSA(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a service account")
		return ctrl.Result{}, err
	}

	if err := r.createDbSecret(ctx, dbbackupcr, dbcr); err != nil {
		log.Error(err, "Couldn't create a secret with credentials")
		return ctrl.Result{}, err
	}

	if err := r.createRole(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a secret with credentials")
		return ctrl.Result{}, err
	}

	if err := r.createRoleBinding(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a secret with credentials")
		return ctrl.Result{}, err
	}

	if err := r.createJob(ctx, dbbackupcr); err != nil {
		log.Error(err, "Couldn't create a pod")
		return ctrl.Result{}, err
	}

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
	meta.SetStatusCondition(
		&obj.Status.Conditions,
		metav1.Condition{Type: consts.TYPE_BACKUP_STATUS, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
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

// Remove the resource holder and a finalizer from the DbBackup
func (r *DbBackupReconciler) cleanup(ctx context.Context, obj *kindarocksv1beta1.DbBackup, cm *corev1.ConfigMap) error {
	if err := r.Delete(ctx, cm); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}
	obj.Finalizers = slices.DeleteFunc(obj.Finalizers, func(item string) bool {
		return item == finResourceHolder
	})

	return nil
}

func (r *DbBackupReconciler) createSA(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.Name,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
		},
	}
	if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, sa); err != nil {
		return err
	}
	return nil
}

func (r *DbBackupReconciler) createRole(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.Name,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
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
	if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, role); err != nil {
		return err
	}
	return nil
}

func (r *DbBackupReconciler) createRoleBinding(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	role := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.Name,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			APIGroup:  "",
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     obj.Name,
		},
	}
	if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, role); err != nil {
		return err
	}
	return nil
}

func (r *DbBackupReconciler) createDbSecret(ctx context.Context, obj *kindarocksv1beta1.DbBackup, dbcr *kindarocksv1beta1.Database) error {
	immutable := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.Name,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
		},
		Immutable: &immutable,
		Data:      map[string][]byte{},
	}

	// Getting credentials for performing a backup
	dbSecret, err := r.getDatabaseSecret(ctx, dbcr)
	if err != nil {
		return err
	}

	adminSecret, err := r.getAdminSecret(ctx, dbcr)
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

	adminCred, err := db.ParseAdminCredentials(ctx, adminSecret.Data)
	if err != nil {
		return err
	}

	envData := map[string][]byte{}
	switch dbcr.Status.Engine {
	case "postgres":
		envData["PGHOST"] = []byte(db.GetDatabaseAddress(ctx).Host)
		envData["PGPORT"] = []byte(strconv.FormatUint(uint64(db.GetDatabaseAddress(ctx).Port), 10))
		envData["PGDATABASE"] = []byte(databaseCred.DatabaseName)
		envData["PGPASSWORD"] = []byte(adminCred.Password)
		envData["PGUSER"] = []byte(adminCred.Username)
	case "mysql":
		return errors.New("not implemented")
	default:
		return errors.New("not supported engine type")
	}

	secret.Data = envData

	if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, secret); err != nil {
		return err
	}

	return nil
}

func (r *DbBackupReconciler) createJob(ctx context.Context, obj *kindarocksv1beta1.DbBackup) error {
	var parallelism int32 = 1

	var selectorLabels map[string]string
	if len(obj.Labels) > 0 {
		selectorLabels = obj.DeepCopy().Labels
	} else {
		selectorLabels = map[string]string{}
	}
	selectorLabels["app.kubernetes.io/created-by"] = "db-operator"
	selectorLabels["app.kubernetes.io/managed-by"] = obj.Name
	selectorLabels["app.kubernetes.io/instance"] = obj.Name

	hostUser := false
	image := fmt.Sprintf("%s/%s:%s", *obj.Spec.Image.Registry, *obj.Spec.Image.Repository, *obj.Spec.Image.Tag)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.Name,
			Namespace:   obj.Namespace,
			Labels:      obj.Labels,
			Annotations: obj.Annotations,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			BackoffLimit: obj.Spec.Retries,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      selectorLabels,
					Annotations: obj.Annotations,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "backup",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "backup",
						Image: image,
						Args:  []string{"backup"},
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: fmt.Sprintf("%s-%s", obj.Namespace, obj.Name),
								},
							},
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "backup",
							MountPath: "/backup",
						}},
						ImagePullPolicy: corev1.PullPolicy(*obj.Spec.Image.PullPolicy),
					}, {}},
					ServiceAccountName: fmt.Sprintf("%s-%s", obj.Namespace, obj.Name),
					SecurityContext:    &corev1.PodSecurityContext{},
					RuntimeClassName:   new(string),
					HostUsers:          &hostUser,
				},
			},
		},
	}

	if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, job); err != nil {
		return err
	}

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

func (r *DbBackupReconciler) getAdminSecret(ctx context.Context, dbcr *kindarocksv1beta1.Database) (*corev1.Secret, error) {
	instance := &kindarocksv1beta1.DbInstance{}
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
