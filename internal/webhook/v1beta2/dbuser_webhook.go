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
	"errors"
	"fmt"
	"slices"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kindarocksv1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	"github.com/db-operator/db-operator/v2/pkg/consts"
)

// nolint:unused
// log is for logging in this package.
var dbuserlog = logf.Log.WithName("dbuser-resource")

// SetupDbUserWebhookWithManager registers the webhook for DbUser in the manager.
func SetupDbUserWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kindarocksv1beta2.DbUser{}).
		WithValidator(&DbUserCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-kinda-rocks-v1beta2-dbuser,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbusers,verbs=create;update,versions=v1beta2,name=vdbuser-v1beta2.kb.io,admissionReviewVersions=v1

type DbUserCustomValidator struct{}

func TestExtraPrivileges(privileges []string) error {
	for _, privilege := range privileges {
		if strings.ToUpper(privilege) == consts.ALL_PRIVILEGES {
			return errors.New("it's not allowed to grant ALL PRIVILEGES")
		}
	}
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type DbUser.
func (v *DbUserCustomValidator) ValidateCreate(_ context.Context, obj *kindarocksv1beta2.DbUser) (admission.Warnings, error) {
	dbuserlog.Info("Validation for DbUser upon creation", "name", obj.GetName())

	warnings := []string{}
	if len(obj.Spec.ExtraPrivileges) > 0 {
		warnings = append(warnings,
			"extra privileges is an experimental feature, please use at your own risk and feel free to open GitHub issues.")
	}
	if err := TestExtraPrivileges(obj.Spec.ExtraPrivileges); err != nil {
		return warnings, err
	}
	if err := kindarocksv1beta2.IsAccessTypeSupported(obj.Spec.AccessType); err != nil {
		return warnings, err
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type DbUser.
func (v *DbUserCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kindarocksv1beta2.DbUser) (admission.Warnings, error) {
	dbuserlog.Info("Validation for DbUser upon update", "name", newObj.GetName())

	warnings := []string{}
	if len(newObj.Spec.ExtraPrivileges) > 0 {
		warnings = append(warnings,
			"extra privileges is an experimental feature, please use at your own risk and feel free to open GitHub issues.")
	}
	if err := TestExtraPrivileges(newObj.Spec.ExtraPrivileges); err != nil {
		return warnings, err
	}

	if err := kindarocksv1beta2.IsAccessTypeSupported(newObj.Spec.AccessType); err != nil {
		return warnings, err
	}
	if newObj.Spec.Credentials.Templates != nil {
		if err := ValidateTemplates(newObj.Spec.Credentials.Templates, false); err != nil {
			return warnings, err
		}
	}
	if oldObj.Spec.Postgres.GrantToAdmin != newObj.Spec.Postgres.GrantToAdmin {
		return warnings, errors.New("grantToAdmin is an immutable field")
	}
	for _, role := range oldObj.Spec.ExtraPrivileges {
		if !slices.Contains(newObj.Spec.ExtraPrivileges, role) {
			warnings = append(
				warnings,
				fmt.Sprintf("extra privileges can't be removed by the operator, please manually revoke %s from the user %s",
					role, newObj.GetName()),
			)
		}
	}

	return warnings, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type DbUser.
func (v *DbUserCustomValidator) ValidateDelete(_ context.Context, obj *kindarocksv1beta2.DbUser) (admission.Warnings, error) {
	dbuserlog.Info("Validation for DbUser upon deletion", "name", obj.GetName())
	return nil, nil
}
