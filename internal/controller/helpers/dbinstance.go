// Copyright 2026 DB-Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers is supposed to be used internally by controllers
package helpers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckGrantRules checks if the given namespace matches the regex defined in the GrantRule.
func CheckGrantRules(rule *kindav1.DbInstanceGrantRule, namespace string) (bool, error) {
	re, err := regexp.Compile(rule.NamespaceRegex)
	if err != nil {
		return false, err
	}
	return re.MatchString(namespace), nil
}

// CheckBackupRules checks if the given namespace matches the regex defined in the GrantRule.
func CheckBackupRules(rule *kindav1.DbInstanceBackupRule, namespace string) (bool, error) {
	re, err := regexp.Compile(rule.NamespaceRegex)
	if err != nil {
		return false, err
	}
	return re.MatchString(namespace), nil
}

// CheckNamespaceFilter checks if the given namespace matches any of the regex defined in the DbInstance's NamespaceFilters.
func CheckNamespaceFilter(filters []string, namespace string) (bool, error) {
	if len(filters) == 0 {
		return true, nil
	}
	for _, filter := range filters {
		re, err := regexp.Compile(filter)
		if err != nil {
			return false, err
		}

		if re.MatchString(namespace) {
			return true, nil
		}
	}
	return false, nil
}

// GetValueFromSource retrieves the value from a ValueSource, which can be either a direct value or a reference to a Secret or ConfigMap.
func GetValueFromSource(ctx context.Context, client client.Client, valueFrom *kindav1.ValueSource) (string, error) {
	if valueFrom == nil {
		return "", errors.New("valueFrom can't be nil")
	}

	if valueFrom.Value != nil {
		return *valueFrom.Value, nil
	}

	if valueFrom.ValueFrom != nil {
		if valueFrom.ValueFrom.SecretKeyRef != nil {
			secretRef := valueFrom.ValueFrom.SecretKeyRef
			secret := &corev1.Secret{}
			err := client.Get(ctx, types.NamespacedName{Namespace: *secretRef.Namespace, Name: *secretRef.Name}, secret)
			if err != nil {
				return "", err
			}
			if val, ok := secret.Data[*secretRef.Key]; ok {
				return string(val), nil
			}
			return "", fmt.Errorf("key not found in secret: %s", *secretRef.Key)
		} else if valueFrom.ValueFrom.ConfigMapKeyRef != nil {
			cmRef := valueFrom.ValueFrom.ConfigMapKeyRef
			cm := &corev1.ConfigMap{}
			err := client.Get(ctx, types.NamespacedName{Namespace: *cmRef.Namespace, Name: *cmRef.Name}, cm)
			if err != nil {
				return "", err
			}
			if val, ok := cm.Data[*cmRef.Key]; ok {
				return string(val), nil
			}
			return "", fmt.Errorf("key not found in configmap: %s", *cmRef.Key)
		}
		return "", errors.New("valueFrom must have either secretRef or configMapRef")
	}

	return "", errors.New("valueSource must have either value or valueFrom")
}

// GetResourceFromValueSource returns an object that is referenced by a ValueSource, which can be either a Secret or a ConfigMap.
func GetResourceFromValueSource(ctx context.Context, client client.Client, valueFrom *kindav1.ValueFrom) (client.Object, error) {
	if valueFrom == nil {
		return nil, errors.New("valueFrom can't be nil")
	}

	if valueFrom.SecretKeyRef != nil {
		if valueFrom.SecretKeyRef.Namespace == nil || valueFrom.SecretKeyRef.Name == nil {
			return nil, errors.New("secretKeyRef must have both namespace and name")
		}
		secret := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Namespace: *valueFrom.SecretKeyRef.Namespace, Name: *valueFrom.SecretKeyRef.Name}, secret)
		if err != nil {
			return nil, err
		}
		return secret, nil
	} else if valueFrom.ConfigMapKeyRef != nil {

		if valueFrom.ConfigMapKeyRef.Namespace == nil || valueFrom.ConfigMapKeyRef.Name == nil {
			return nil, errors.New("configMapKeyRef must have both namespace and name")
		}
		cm := &corev1.ConfigMap{}
		err := client.Get(ctx, types.NamespacedName{Namespace: *valueFrom.ConfigMapKeyRef.Namespace, Name: *valueFrom.ConfigMapKeyRef.Name}, cm)
		if err != nil {
			return nil, err
		}
		return cm, nil
	}

	return nil, errors.New("valueFrom must have either secretKeyRef or configMapKeyRef")
}

// ObjectMetadataFormat returns a string representation of the object metadata in the format "kind/namespace/name".
func ObjectMetadataFormat(obj client.Object) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
}

// ObjectFromFormattedString parses a string in the format "kind/namespace/name" and returns the corresponding object.
func ObjectFromFormattedString(entry string) (client.Object, error) {
	parts := strings.Split(entry, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid resource entry: %q", entry)
	}

	kind, namespace, name := parts[0], parts[1], parts[2]
	var obj client.Object
	switch kind {
	case "ConfigMap":
		obj = &corev1.ConfigMap{}
	case "Secret":
		obj = &corev1.Secret{}
	default:
		return nil, fmt.Errorf("unsupported resource kind %q", kind)
	}

	obj.SetNamespace(namespace)
	obj.SetName(name)

	return obj, nil
}
