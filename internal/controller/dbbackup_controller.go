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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kindarocksv1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/helpers/tplrender"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// DbBackupReconciler reconciles a DbBackup object
type DbBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Opts   *DbBackupReconcilerOpts
}

// Options for the DbBackupReconciler
type DbBackupReconcilerOpts struct {
	// A path to a directory with manifests templates
	TemplatesDir string
}

// Definitions to manage status conditions
const (
	// typeObjectStatus represents the status of the whole CR
	typeObjectStatus = "ObjectStatus"
)

// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kinda.rocks,resources=dbbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,configmaps,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DbBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	log := logf.FromContext(ctx)

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
		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: typeObjectStatus, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
		)

		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_BACKUP_READY, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
		)

		meta.SetStatusCondition(
			&dbbackupcr.Status.Conditions,
			metav1.Condition{Type: consts.TYPE_UPLOAD_READ, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"},
		)

		if err = r.Status().Update(ctx, dbbackupcr); err != nil {
			log.Error(err, "Failed to update DbBackup status")
			return ctrl.Result{}, err
		}
		// No need for retry, since the status was updated
		return ctrl.Result{}, err
	}

	// If object status is false, do not retry
	if meta.IsStatusConditionFalse(dbbackupcr.Status.Conditions, typeObjectStatus) {
		log.Info("The object status is false, not retrying")
		return ctrl.Result{}, nil
	}

	// If not backup.success, create a backup pod
	if !meta.IsStatusConditionPresentAndEqual(dbbackupcr.Status.Conditions, consts.TYPE_BACKUP_READY, metav1.ConditionUnknown) {
		if *dbbackupcr.Status.Backup.FailedRetries >= *dbbackupcr.Spec.Retries {
			err := errors.New("failed retries amount is reached")
			log.Error(err, "The amount of  failed retries is reached, CR is marked as failed", "retry", dbbackupcr.Status.Backup.FailedRetries, "op", "backup")

			meta.SetStatusCondition(
				&dbbackupcr.Status.Conditions,
				metav1.Condition{Type: typeObjectStatus, Status: metav1.ConditionFalse, Reason: "Failed", Message: "Unable to perform a backup"},
			)

			if err = r.Status().Update(ctx, dbbackupcr); err != nil {
				log.Error(err, "Failed to update DbBackup status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Info("Executing the backup logic", "retry", dbbackupcr.Status.Backup.FailedRetries)

		roleTplRaw, err := tplrender.ReadFile(r.Opts.TemplatesDir, "role.gotmpl")
		if err != nil {
			log.Error(err, "Couldn't read a template", "template", "role.gotmpl")
			return ctrl.Result{}, err
		}
		roleTpl, err := tplrender.BuildTpl(roleTemplatePath)
		// Create a Role and a RoleBinding from template
		// Create a ServiceAccount from a template
		// Create a Pod from template
		// Create a backup pod and quit.
		// The backup pod should change the status of the DbBackup object and trigger a new reconciliaion
		return ctrl.Result{}, nil
	}

	// If not upload.success, create an upload pod
	if !meta.IsStatusConditionPresentAndEqual(dbbackupcr.Status.Conditions, consts.TYPE_UPLOAD_READ, metav1.ConditionUnknown) {
		if *dbbackupcr.Status.Backup.FailedRetries >= *dbbackupcr.Spec.Retries {
			err := errors.New("failed retries amount is reached")
			log.Error(err, "The amount of failed retries is reached, CR is marked as failed", "retry", dbbackupcr.Status.Upload.FailedRetries, "op", "upload")
			meta.SetStatusCondition(
				&dbbackupcr.Status.Conditions,
				metav1.Condition{Type: typeObjectStatus, Status: metav1.ConditionFalse, Reason: "Failed", Message: "Unable to upload a backup"},
			)
			if err = r.Status().Update(ctx, dbbackupcr); err != nil {
				log.Error(err, "Failed to update DbBackup status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Info("Executing the upload logic", "retry", dbbackupcr.Status.Backup.FailedRetries)
		// Create an upload pod and quit.
		// The upload pod should change the status of the DbBackup object and trigger a new reconciliaion
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
