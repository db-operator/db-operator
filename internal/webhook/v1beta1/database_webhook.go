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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	return ctrl.NewWebhookManagedBy(mgr).For(&kindarocksv1beta1.Database{}).
		WithValidator(&DatabaseCustomValidator{}).
		WithDefaulter(&DatabaseCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kinda-rocks-v1beta1-database,mutating=true,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=databases,verbs=create;update,versions=v1beta1,name=mdatabase-v1beta1.kb.io,admissionReviewVersions=v1

type DatabaseCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &DatabaseCustomDefaulter{}

func (d *DatabaseCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	database, ok := obj.(*kindarocksv1beta1.Database)

	if !ok {
		return fmt.Errorf("expected an Database object but got %T", obj)
	}

	databaselog.Info("Defaulting for Database", "name", database.GetName())

	if len(database.Spec.SecretsTemplates) == 0 && len(database.Spec.Credentials.Templates) == 0 {
		database.Spec.Credentials = kindarocksv1beta1.Credentials{
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

var _ webhook.CustomValidator = &DatabaseCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	database, ok := obj.(*kindarocksv1beta1.Database)
	if !ok {
		return nil, fmt.Errorf("expected a Database object but got %T", obj)
	}
	databaselog.Info("Validation for Database upon creation", "name", database.GetName())

	var warnings []string
	// TODO(user): fill in your validation logic upon object creation.
	if database.Spec.SecretsTemplates != nil && database.Spec.Credentials.Templates != nil {
		return nil, errors.New("using both: secretsTemplates and templates, is not allowed")
	}

	if database.Spec.SecretsTemplates != nil {
		warnings = append(warnings, "secretsTemplates are deprecated, it will be removed in the next API version. Please, consider switching to templates")
		// TODO: Migrate this logic to the webhook package
		if err := ValidateSecretTemplates(database.Spec.SecretsTemplates); err != nil {
			return warnings, err
		}
	}

	if database.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(database.Spec.Credentials.Templates, true); err != nil {
			return warnings, err
		}
	}

	for _, extraGrant := range database.Spec.ExtraGrants {
		if err := kindarocksv1beta1.IsAccessTypeSupported(extraGrant.AccessType); err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	database, ok := newObj.(*kindarocksv1beta1.Database)
	if !ok {
		return nil, fmt.Errorf("expected a Database object for the newObj but got %T", newObj)
	}
	databaselog.Info("Validation for Database upon update", "name", database.GetName())

	if database.Spec.SecretsTemplates != nil && database.Spec.Credentials.Templates != nil {
		return nil, errors.New("using both: secretsTemplates and templates, is not allowed")
	}

	var warnings []string

	if database.Spec.SecretsTemplates != nil {
		warnings = append(warnings, "secretsTemplates are deprecated, it will be removed in the next API version. Please, consider switching to templates")
		err := ValidateSecretTemplates(database.Spec.SecretsTemplates)
		if err != nil {
			return warnings, err
		}
	}

	if database.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(database.Spec.Credentials.Templates, true); err != nil {
			return warnings, err
		}
	}

	// Ensure fields are immutable
	immutableErr := "cannot change %s, the field is immutable"
	oldDatabase, _ := oldObj.(*kindarocksv1beta1.Database)
	if database.Spec.Instance != oldDatabase.Spec.Instance {
		return warnings, fmt.Errorf(immutableErr, "spec.instance")
	}

	if database.Spec.Postgres.Template != oldDatabase.Spec.Postgres.Template {
		return warnings, fmt.Errorf(immutableErr, "spec.postgres.template")
	}

	return warnings, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	database, ok := obj.(*kindarocksv1beta1.Database)
	if !ok {
		return nil, fmt.Errorf("expected a Database object but got %T", obj)
	}
	databaselog.Info("Validation for Database upon deletion", "name", database.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
