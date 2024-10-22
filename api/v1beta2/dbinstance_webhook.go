/*
 * Copyright 2023 DB-Operator Authors
 *
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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/db-operator/db-operator/internal/helpers/kube"
	"github.com/db-operator/db-operator/pkg/consts"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var dbinstancelog = logf.Log.WithName("dbinstance-resource")

func (r *DbInstance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-kinda-rocks-v1beta2-dbinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=kinda.rocks,resources=dbinstances,verbs=create;update,versions=v1beta2,name=vdbinstance.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DbInstance{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DbInstance) Default() {
	dbinstancelog.Info("default", "name", r.Name)
}

func TestAllowedPrivileges(privileges []string) error {
	for _, privilege := range privileges {
		if strings.ToUpper(privilege) == consts.ALL_PRIVILEGES {
			return errors.New("it's not allowed to grant ALL PRIVILEGES")
		}
	}
	return nil
}

func (r *DbInstance) ValidateCreate() (admission.Warnings, error) {
	if err := TestAllowedPrivileges(r.Spec.AllowedPrivileges); err != nil {
		return nil, err
	}

	dbinstancelog.Info("validate create", "name", r.Name)
	if err := ValidateConfigVsConfigFrom(r.Spec.InstanceData); err != nil {
		return nil, err
	}
	if err := ValidateEngine(r.Spec.Engine); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DbInstance) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := TestAllowedPrivileges(r.Spec.AllowedPrivileges); err != nil {
		return nil, err
	}

	// Once connection data is changed, all the databases will be reconciled with newer
	// db-instance host, that can break all the applications that are using that dbinstance
	if old.(*DbInstance).Status.Connected {
		if !reflect.DeepEqual(old.(*DbInstance).Spec.InstanceData, r.Spec.InstanceData) {
			allowMigration, ok := old.(*DbInstance).ObjectMeta.Annotations[consts.DBINSTANCE_ALLOW_MIGRATION]
			if !ok || allowMigration != "true" {
				return nil, fmt.Errorf(
					"to change the connection data of an already connected instance, set the %s annotation to 'true'",
					consts.DBINSTANCE_ALLOW_MIGRATION,
				)
			}
		}
	}
	dbinstancelog.Info("validate update", "name", r.Name)
	immutableErr := "cannot change %s, the field is immutable"
	if r.Spec.Engine != old.(*DbInstance).Spec.Engine {
		return nil, fmt.Errorf(immutableErr, "engine")
	}

	return nil, nil
}

func ValidateConfigVsConfigFrom(r *InstanceData) error {
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

func ValidateConfigFrom(dbin *InstanceData) error {
	check := dbin.HostFrom
	if check != nil && !(check.Kind == kube.CONFIGMAP || check.Kind == kube.SECRET) {
		return fmt.Errorf("unsupported kind in hostFrom: %s, please use %s or %s", check.Kind, kube.CONFIGMAP, kube.SECRET)
	}
	check = dbin.PortFrom
	if check != nil && !(check.Kind == kube.CONFIGMAP || check.Kind == kube.SECRET) {
		return fmt.Errorf("unsupported kind in portFrom: %s, please use %s or %s", check.Kind, kube.CONFIGMAP, kube.SECRET)
	}
	return nil
}

func ValidateEngine(engine Engine) error {
	if !(slices.Contains([]Engine{"postgres", "mysql"}, engine)) {
		return fmt.Errorf("unsupported engine: %s. please use either postgres or mysql", engine)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DbInstance) ValidateDelete() (admission.Warnings, error) {
	dbinstancelog.Info("validate delete", "name", r.Name)
	return nil, nil
}
