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

package v1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kindarocksv1 "github.com/db-operator/db-operator/v2/api/v1"
)

// nolint:unused
// log is for logging in this package.
var dbinstancelog = logf.Log.WithName("dbinstance-resource")

// SetupDbInstanceWebhookWithManager registers the webhook for DbInstance in the manager.
func SetupDbInstanceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kindarocksv1.DbInstance{}).
		WithValidator(&DbInstanceCustomValidator{}).
		WithDefaulter(&DbInstanceCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-kinda-rocks-v1-dbinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbinstances,verbs=create;update,versions=v1,name=mdbinstance-v1.kb.io,admissionReviewVersions=v1

// DbInstanceCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind DbInstance when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type DbInstanceCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind DbInstance.
func (d *DbInstanceCustomDefaulter) Default(_ context.Context, obj *kindarocksv1.DbInstance) error {
	dbinstancelog.Info("Defaulting for DbInstance", "name", obj.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-kinda-rocks-v1-dbinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbinstances,verbs=create;update,versions=v1,name=vdbinstance-v1.kb.io,admissionReviewVersions=v1

// DbInstanceCustomValidator struct is responsible for validating the DbInstance resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type DbInstanceCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateCreate(_ context.Context, obj *kindarocksv1.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kindarocksv1.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateDelete(_ context.Context, obj *kindarocksv1.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
