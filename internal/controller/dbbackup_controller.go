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
	"strconv"

	dbhelper "github.com/db-operator/db-operator/v2/internal/helpers/database"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kindarocksv1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/helpers/database"
	kubehelper "github.com/db-operator/db-operator/v2/internal/helpers/kube"
	"github.com/db-operator/db-operator/v2/internal/helpers/tplrender"
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
	TemplatesDir string
	kubeHelper   *kubehelper.KubeHelper
	Namespace    string
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
func (r *DbBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// If status conditions are not set, it probably means that the CR was just created.
	// Set the status conditions and return to let the next reconcile loop continue the logic
	if len(dbbackupcr.Status.Conditions) == 0 {
		log.Info("Initializing an object")
		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_BACKUP_STATUS, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
		)
		lockedByBackupPod := false
		dbbackupcr.Status.LockedByBackupPod = &lockedByBackupPod
		if err = r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Failed to update DbBackup status")
			return ctrl.Result{}, err
		}
		// No need for retry, since the status was updated
		return ctrl.Result{}, err
	}

	// If object status is false, do not retry
	if meta.IsStatusConditionFalse(dbbackupcr.Status.Conditions, consts.TYPE_BACKUP_STATUS) {
		log.Info("The object status is false, not retrying")
		return ctrl.Result{}, nil
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
	if meta.IsStatusConditionPresentAndEqual(dbbackupcr.Status.Conditions, consts.TYPE_BACKUP_STATUS, metav1.ConditionUnknown) {
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
		r.Opts.kubeHelper = kubehelper.NewKubeHelper(r.Client, r.Recorder, dbbackupcr)

		tplData := &tplrender.TplData{
			Namespace:       r.Opts.Namespace,
			Engine:          dbcr.Status.Engine,
			ImageRegistry:   *dbbackupcr.Spec.Image.Registry,
			ImageRepository: *dbbackupcr.Spec.Image.Repository,
			ImageTag:        *dbbackupcr.Spec.Image.Tag,
			ImagePullPolicy: *dbbackupcr.Spec.Image.PullPolicy,
			DatabaseName:    dbcr.Name,
		}

		var template string
		template = "role.gotmpl"

		codecs := serializer.NewCodecFactory(r.Scheme)
		decoder := codecs.UniversalDeserializer()

		roleManifest, err := r.buildObjFromTemplate(template, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", template)
			return ctrl.Result{}, nil
		}

		role := &rbacv1.Role{}
		_, _, err = decoder.Decode(roleManifest, nil, role)
		if err != nil {
			return ctrl.Result{}, err
		}

		role.Name = dbbackupcr.Name
		role.Namespace = r.Opts.Namespace
		role.Labels = dbbackupcr.Labels
		role.Annotations = dbbackupcr.Annotations

		requiredRules := []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets", "configmaps"},
				ResourceNames: []string{dbcr.Spec.SecretName},
			},
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{dbcr.GroupVersionKind().Group},
				Resources:     []string{"databases", "dbbackups"},
				ResourceNames: []string{dbcr.Name},
			},
			{
				Verbs:         []string{"get", "update", "patch"},
				APIGroups:     []string{dbbackupcr.GroupVersionKind().Group},
				Resources:     []string{"dbbackups/status"},
				ResourceNames: []string{dbbackupcr.Name},
			},
		}

		if len(role.Rules) == 0 {
			role.Rules = []rbacv1.PolicyRule{}
		}

		role.Rules = append(role.Rules, requiredRules...)

		if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, role); err != nil {
			log.Error(err, "Couldn't create a role")
			return ctrl.Result{}, nil
		}

		template = "service_account.gotmpl"

		saManifest, err := r.buildObjFromTemplate(template, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", template)
			return ctrl.Result{}, nil
		}

		sa := &v1.ServiceAccount{}
		_, _, err = decoder.Decode(saManifest, nil, sa)
		if err != nil {
			return ctrl.Result{}, err
		}
		sa.Name = dbbackupcr.Name
		sa.Namespace = r.Opts.Namespace
		sa.Labels = dbbackupcr.Labels
		sa.Annotations = dbbackupcr.Annotations

		if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, sa); err != nil {
			log.Error(err, "Couldn't create a role")
			return ctrl.Result{}, nil
		}

		template = "role_binding.gotmpl"
		rbManifest, err := r.buildObjFromTemplate(template, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", template)
			return ctrl.Result{}, nil
		}

		rb := &rbacv1.RoleBinding{}
		_, _, err = decoder.Decode(rbManifest, nil, rb)
		rb.Name = dbbackupcr.Name
		rb.Namespace = r.Opts.Namespace
		rb.Labels = dbbackupcr.Labels
		rb.Annotations = dbbackupcr.Annotations

		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: role.GroupVersionKind().Group,
			Kind:     "Role",
			Name:     role.GetName(),
		}
		rb.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			APIGroup:  sa.GroupVersionKind().Group,
			Name:      sa.GetName(),
			Namespace: sa.GetNamespace(),
		}}
		if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, rb); err != nil {
			log.Error(err, "Couldn't create a role")
			return ctrl.Result{}, nil
		}

		template = "secret.gotmpl"
		secretBackupEnvManifest, err := r.buildObjFromTemplate(template, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", template)
			return ctrl.Result{}, nil
		}
		secretBackupEnv := &v1.Secret{}
		_, _, err = decoder.Decode(secretBackupEnvManifest, nil, secretBackupEnv)

		// Getting credentials for performing a backup
		secret, err := r.getDatabaseSecret(ctx, dbcr)
		if err != nil {
			log.Error(err, "Couldn't get a database secret", "namespace", dbcr.Namespace, "name", dbcr.Spec.SecretName)
			return ctrl.Result{}, err
		}

		adminSecret, err := r.getAdminSecret(ctx, dbcr)
		if err != nil {
			log.Error(err, "Couldn't get the admin secret")
			return ctrl.Result{}, err
		}
		// TODO: All these methods should be somehow more accessible
		databaseCred, err := dbhelper.ParseDatabaseSecretData(dbcr, secret.Data)
		if err != nil {
			// failed to parse database credential from secret
			return ctrl.Result{}, err
		}
		instance := &kindarocksv1beta1.DbInstance{}
		if err := r.Get(ctx, types.NamespacedName{Name: dbcr.Spec.Instance}, instance); err != nil {
			return ctrl.Result{}, err
		}
		db, _, err := database.FetchDatabaseData(ctx, dbcr, databaseCred, instance)

		adminCred, err := db.ParseAdminCredentials(ctx, adminSecret.Data)
		if err != nil {
			// failed to parse database admin secret
			return ctrl.Result{}, err
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
			log.Info("Not yet there")
		default:
			return ctrl.Result{}, errors.New("not supported engine type")
		}

		if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, secretBackupEnv); err != nil {
			log.Error(err, "Couldn't create a secrert")
			return ctrl.Result{}, err
		}

		template = "pod.gotmpl"
		podManifest, err := r.buildObjFromTemplate(template, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", template)
			return ctrl.Result{}, nil
		}
		pod := &v1.Pod{}
		_, _, err = decoder.Decode(podManifest, nil, pod)
		pod.Name = dbbackupcr.Name
		pod.Namespace = r.Opts.Namespace
		pod.Labels = dbbackupcr.Labels
		pod.Annotations = dbbackupcr.Annotations
		pod.Spec.ServiceAccountName = sa.Name

		for i, container := range pod.Spec.Containers {
			if container.Name == "backup" {
				pod.Spec.Containers[i].EnvFrom = []v1.EnvFromSource{{
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: secretBackupEnv.Name,
						},
					},
				}}
			}
		}

		if err := r.Opts.kubeHelper.HandleCreateOrUpdate(ctx, pod); err != nil {
			log.Error(err, "Couldn't create a pod")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, nil
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

// Build a runtime object from a template
func (r *DbBackupReconciler) buildObjFromTemplate(template string, data *tplrender.TplData) ([]byte, error) {
	tplRaw, err := tplrender.ReadFile(r.Opts.TemplatesDir, template)
	if err != nil {
		return nil, err
	}
	tpl, err := tplrender.Render(tplRaw, data)
	if err != nil {
		return nil, err
	}

	return tpl, nil
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
