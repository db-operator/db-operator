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

package v1beta1

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kindarocksv1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var databaselog = logf.Log.WithName("database-resource")

const (
	DEFAULT_TEMPLATE_VALUE = "{{ .Protocol }}://{{ .Username }}:{{ .Password }}@{{ .Hostname }}:{{ .Port }}/{{ .Database }}"
	DEFAULT_TEMPLATE_NAME  = "CONNECTION_STRING"
)

// SetupDatabaseWebhookWithManager registers the webhook for Database in the manager.
func SetupDatabaseWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kindarocksv1beta1.Database{}).
		WithValidator(&DatabaseCustomValidator{}).
		WithDefaulter(&DatabaseCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kinda-rocks-v1beta1-database,mutating=true,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=databases,verbs=create;update,versions=v1beta1,name=mdatabase-v1beta1.kb.io,admissionReviewVersions=v1

type DatabaseCustomDefaulter struct{}

func (d *DatabaseCustomDefaulter) Default(_ context.Context, obj *kindarocksv1beta1.Database) error {
	databaselog.Info("Defaulting for Database", "name", obj.GetName())

	if len(obj.Spec.SecretsTemplates) == 0 && len(obj.Spec.Credentials.Templates) == 0 {
		obj.Spec.Credentials = kindarocksv1beta1.Credentials{
			Templates: kindarocksv1beta1.Templates{
				&kindarocksv1beta1.Template{
					Name:     DEFAULT_TEMPLATE_NAME,
					Template: DEFAULT_TEMPLATE_VALUE,
					Secret:   true,
				},
			},
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-kinda-rocks-v1beta1-database,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=databases,verbs=create;update,versions=v1beta1,name=vdatabase-v1beta1.kb.io,admissionReviewVersions=v1

type DatabaseCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateCreate(_ context.Context, obj *kindarocksv1beta1.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon creation", "name", obj.GetName())

	var warnings []string
	// TODO(user): fill in your validation logic upon object creation.
	if obj.Spec.SecretsTemplates != nil && obj.Spec.Credentials.Templates != nil {
		return nil, errors.New("using both: secretsTemplates and templates, is not allowed")
	}

	if obj.Spec.SecretsTemplates != nil {
		warnings = append(warnings, "secretsTemplates are deprecated, it will be removed in the next API version. Please, consider switching to templates")
		// TODO: Migrate this logic to the webhook package
		if err := ValidateSecretTemplates(obj.Spec.SecretsTemplates); err != nil {
			return warnings, err
		}
	}

	if obj.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(obj.Spec.Credentials.Templates, true); err != nil {
			return warnings, err
		}
	}

	for _, extraGrant := range obj.Spec.ExtraGrants {
		if err := kindarocksv1beta1.IsAccessTypeSupported(extraGrant.AccessType); err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kindarocksv1beta1.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon update", "name", newObj.GetName())

	if newObj.Spec.SecretsTemplates != nil && newObj.Spec.Credentials.Templates != nil {
		return nil, errors.New("using both: secretsTemplates and templates, is not allowed")
	}

	var warnings []string

	if newObj.Spec.SecretsTemplates != nil {
		warnings = append(warnings, "secretsTemplates are deprecated, it will be removed in the next API version. Please, consider switching to templates")
		err := ValidateSecretTemplates(newObj.Spec.SecretsTemplates)
		if err != nil {
			return warnings, err
		}
	}

	if newObj.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(newObj.Spec.Credentials.Templates, true); err != nil {
			return warnings, err
		}
	}

	if len(oldObj.Spec.ExistingUser) > 0 && len(newObj.Spec.ExistingUser) == 0 {
		warnings = append(warnings, "After swtching from exsting user to a generated user, the password is set to an empty string, remove the db secret to generate it")
	}

	// Ensure fields are immutable
	immutableErr := "cannot change %s, the field is immutable"
	if newObj.Spec.Instance != oldObj.Spec.Instance {
		return warnings, fmt.Errorf(immutableErr, "spec.instance")
	}

	if newObj.Spec.Postgres.Template != oldObj.Spec.Postgres.Template {
		return warnings, fmt.Errorf(immutableErr, "spec.postgres.template")
	}

	return warnings, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateDelete(ctx context.Context, obj *kindarocksv1beta1.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
