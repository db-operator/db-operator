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
	"reflect"
	"slices"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kindarocksv1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	"github.com/db-operator/db-operator/v2/internal/helpers/kube"
	"github.com/db-operator/db-operator/v2/pkg/consts"
)

// nolint:unused
// log is for logging in this package.
var dbinstancelog = logf.Log.WithName("dbinstance-resource")

// SetupDbInstanceWebhookWithManager registers the webhook for DbInstance in the manager.
func SetupDbInstanceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kindarocksv1beta2.DbInstance{}).
		WithValidator(&DbInstanceCustomValidator{}).
		WithDefaulter(&DbInstanceCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-kinda-rocks-v1beta2-dbinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbinstances,verbs=create;update,versions=v1beta2,name=mdbinstance-v1beta2.kb.io,admissionReviewVersions=v1

type DbInstanceCustomDefaulter struct{}

func (d *DbInstanceCustomDefaulter) Default(_ context.Context, obj *kindarocksv1beta2.DbInstance) error {
	dbinstancelog.Info("Defaulting for DbInstance", "name", obj.GetName())
	return nil
}

// +kubebuilder:webhook:path=/validate-kinda-rocks-v1beta2-dbinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbinstances,verbs=create;update,versions=v1beta2,name=vdbinstance-v1beta2.kb.io,admissionReviewVersions=v1

type DbInstanceCustomValidator struct{}

func TestAllowedPrivileges(privileges []string) error {
	for _, privilege := range privileges {
		if strings.ToUpper(privilege) == consts.ALL_PRIVILEGES {
			return errors.New("it's not allowed to grant ALL PRIVILEGES")
		}
	}
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateCreate(_ context.Context, obj *kindarocksv1beta2.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon creation", "name", obj.GetName())

	if err := TestAllowedPrivileges(obj.Spec.AllowedPrivileges); err != nil {
		return nil, err
	}
	if err := ValidateConfigVsConfigFrom(obj.Spec.InstanceData); err != nil {
		return nil, err
	}
	if err := ValidateEngine(obj.Spec.Engine); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *kindarocksv1beta2.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon update", "name", newObj.GetName())

	if err := TestAllowedPrivileges(newObj.Spec.AllowedPrivileges); err != nil {
		return nil, err
	}

	// Once connection data is changed, all the databases will be reconciled with newer
	// db-instance host, that can break all the applications that are using that dbinstance
	if oldObj.Status.Connected {
		if !reflect.DeepEqual(oldObj.Spec.InstanceData, newObj.Spec.InstanceData) {
			allowMigration, ok := oldObj.Annotations[consts.DBINSTANCE_ALLOW_MIGRATION]
			if !ok || allowMigration != "true" {
				return nil, fmt.Errorf(
					"to change the connection data of an already connected instance, set the %s annotation to 'true'",
					consts.DBINSTANCE_ALLOW_MIGRATION,
				)
			}
		}
	}

	immutableErr := "cannot change %s, the field is immutable"
	if newObj.Spec.Engine != oldObj.Spec.Engine {
		return nil, fmt.Errorf(immutableErr, "engine")
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type DbInstance.
func (v *DbInstanceCustomValidator) ValidateDelete(_ context.Context, obj *kindarocksv1beta2.DbInstance) (admission.Warnings, error) {
	dbinstancelog.Info("Validation for DbInstance upon deletion", "name", obj.GetName())
	return nil, nil
}

func ValidateConfigVsConfigFrom(r *kindarocksv1beta2.InstanceData) error {
	if r != nil {
		if len(r.Host) > 0 && r.HostFrom != nil {
			return errors.New("it's not allowed to use both host and hostFrom, please choose one")
		}
		if r.Port > 0 && r.PortFrom != nil {
			return errors.New("it's not allowed to use both port and portFrom, please choose one")
		}
	}
	return nil
}

func ValidateConfigFrom(dbin *kindarocksv1beta2.InstanceData) error {
	check := dbin.HostFrom
	if check != nil && check.Kind != kube.CONFIGMAP && check.Kind != kube.SECRET {
		return fmt.Errorf("unsupported kind in hostFrom: %s, please use %s or %s", check.Kind, kube.CONFIGMAP, kube.SECRET)
	}
	check = dbin.PortFrom
	if check != nil && check.Kind != kube.CONFIGMAP && check.Kind != kube.SECRET {
		return fmt.Errorf("unsupported kind in portFrom: %s, please use %s or %s", check.Kind, kube.CONFIGMAP, kube.SECRET)
	}
	return nil
}

func ValidateEngine(engine kindarocksv1beta2.Engine) error {
	supported := []kindarocksv1beta2.Engine{
		kindarocksv1beta2.Engine(consts.ENGINE_POSTGRES),
		kindarocksv1beta2.Engine(consts.ENGINE_MYSQL),
	}
	if !slices.Contains(supported, engine) {
		return fmt.Errorf("unsupported engine: %s. please use one of: %v", engine, supported)
	}
	return nil
}
