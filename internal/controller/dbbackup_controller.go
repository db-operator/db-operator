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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kindarocksv1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
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
			Engine:          dbcr.Status.Engine,
			ImageRegistry:   *dbbackupcr.Spec.Image.Registry,
			ImageRepository: *dbbackupcr.Spec.Image.Repository,
			ImageTag:        *dbbackupcr.Spec.Image.Tag,
		}

		// Prepare a Role
		roleTplRaw, err := tplrender.ReadFile(r.Opts.TemplatesDir, "role.gotmpl")
		if err != nil {
			log.Error(err, "Couldn't read a template", "template", "role.gotmpl")
			return ctrl.Result{}, err
		}
		roleTpl, err := tplrender.Render(roleTplRaw, tplData)
		if err != nil {
			log.Error(err, "Couldn't render a template", "template", "role.gotmpl")
			return ctrl.Result{}, err
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode

		obj, _, err := decode([]byte(roleTpl), nil, nil)
		if err != nil {
			log.Error(err, "Couldn't decode a template into a Kuberntes object", "template", "role.gotmpl")
			return ctrl.Result{}, err
		}

		role := obj.(*rbacv1.Role)
		role.GenerateName = dbbackupcr.Name
		role.Namespace = dbbackupcr.Namespace
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
		// Create a Role and a RoleBinding from template
		// Create a ServiceAccount from a template
		// Create a Pod from template
		// Create a backup pod and quit.
		// The backup pod should change the status of the DbBackup object and trigger a new reconciliaion
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
