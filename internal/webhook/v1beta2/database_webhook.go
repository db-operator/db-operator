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

package v1beta2

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kindarocksv1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
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
	return ctrl.NewWebhookManagedBy(mgr, &kindarocksv1beta2.Database{}).
		WithValidator(&DatabaseCustomValidator{}).
		WithDefaulter(&DatabaseCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kinda-rocks-v1beta2-database,mutating=true,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=databases,verbs=create;update,versions=v1beta2,name=mdatabase-v1beta2.kb.io,admissionReviewVersions=v1

type DatabaseCustomDefaulter struct{}

func (d *DatabaseCustomDefaulter) Default(_ context.Context, obj *kindarocksv1beta2.Database) error {
	databaselog.Info("Defaulting for Database", "name", obj.GetName())

	if len(obj.Spec.Credentials.Templates) == 0 {
		obj.Spec.Credentials = kindarocksv1beta2.Credentials{
			Templates: kindarocksv1beta2.Templates{
				&kindarocksv1beta2.Template{
					Name:     DEFAULT_TEMPLATE_NAME,
					Template: DEFAULT_TEMPLATE_VALUE,
					Secret:   true,
				},
			},
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-kinda-rocks-v1beta2-database,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=databases,verbs=create;update,versions=v1beta2,name=vdatabase-v1beta2.kb.io,admissionReviewVersions=v1

type DatabaseCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateCreate(_ context.Context, obj *kindarocksv1beta2.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon creation", "name", obj.GetName())

	if obj.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(obj.Spec.Credentials.Templates, true); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kindarocksv1beta2.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon update", "name", newObj.GetName())

	if newObj.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(newObj.Spec.Credentials.Templates, true); err != nil {
			return nil, err
		}
	}

	// Ensure fields are immutable
	immutableErr := "cannot change %s, the field is immutable"
	if newObj.Spec.Instance != oldObj.Spec.Instance {
		return nil, fmt.Errorf(immutableErr, "spec.instance")
	}

	if newObj.Spec.Postgres.Params.Template != oldObj.Spec.Postgres.Params.Template {
		return nil, fmt.Errorf(immutableErr, "spec.postgres.params.template")
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateDelete(_ context.Context, obj *kindarocksv1beta2.Database) (admission.Warnings, error) {
	databaselog.Info("Validation for Database upon deletion", "name", obj.GetName())
	return nil, nil
}
