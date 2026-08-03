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

package helpers_test

import (
	"testing"

	"github.com/db-operator/db-operator/v2/internal/controller/helpers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
)

func TestUnitCheckGrantRules(t *testing.T) {
	t.Parallel()
	t.Run("CheckGrantRules Simple string success", func(t *testing.T) {
		grantRule := &kindav1.DbInstanceGrantRule{
			NamespaceRegex: "test-namespace",
			Role:           "test-user",
			AccessLevel:    "readOnly",
		}
		match, err := helpers.CheckGrantRules(grantRule, "test-namespace")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("CheckGrantRules Regex success", func(t *testing.T) {
		grantRule := &kindav1.DbInstanceGrantRule{
			NamespaceRegex: "test-.*",
			Role:           "test-user",
			AccessLevel:    "readOnly",
		}
		match, err := helpers.CheckGrantRules(grantRule, "test-namespace")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("CheckGrantRules Simple string doesn't match", func(t *testing.T) {
		grantRule := &kindav1.DbInstanceGrantRule{
			NamespaceRegex: "test-namespace",
			Role:           "test-user",
			AccessLevel:    "readOnly",
		}
		match, err := helpers.CheckGrantRules(grantRule, "test2-namespace")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("CheckGrantRules Regex doesn't match", func(t *testing.T) {
		grantRule := &kindav1.DbInstanceGrantRule{
			NamespaceRegex: "test-.*",
			Role:           "test-user",
			AccessLevel:    "readOnly",
		}
		match, err := helpers.CheckGrantRules(grantRule, "test2-namespace")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("CheckGrantRules Invalid regex", func(t *testing.T) {
		grantRule := &kindav1.DbInstanceGrantRule{
			NamespaceRegex: "(abv",
			Role:           "test-user",
			AccessLevel:    "readOnly",
		}
		match, err := helpers.CheckGrantRules(grantRule, "test2-namespace")
		assert.Error(t, err)
		assert.False(t, match)
	})
}

func TestUnitCheckBackupRules(t *testing.T) {
	t.Parallel()
	t.Run("CheckBackupRules Simple string success", func(t *testing.T) {
		rule := &kindav1.DbInstanceBackupRule{
			NamespaceRegex: "test-namespace",
			Name:           "test-backup",
			Cron:           "*/15 * * * *",
		}
		match, err := helpers.CheckBackupRules(rule, "test-namespace")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("CheckBackupRules Regex success", func(t *testing.T) {
		rule := &kindav1.DbInstanceBackupRule{
			NamespaceRegex: "test-.*",
			Name:           "test-backup",
			Cron:           "*/15 * * * *",
		}
		match, err := helpers.CheckBackupRules(rule, "test-namespace")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("CheckBackupRules Simple string doesn't match", func(t *testing.T) {
		rule := &kindav1.DbInstanceBackupRule{
			NamespaceRegex: "test-namespace",
			Name:           "test-backup",
			Cron:           "*/15 * * * *",
		}
		match, err := helpers.CheckBackupRules(rule, "test2-namespace")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("CheckBackupRules Regex doesn't match", func(t *testing.T) {
		rule := &kindav1.DbInstanceBackupRule{
			NamespaceRegex: "test-.*",
			Name:           "test-backup",
			Cron:           "*/15 * * * *",
		}
		match, err := helpers.CheckBackupRules(rule, "test2-namespace")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("CheckGrantRules Invalid regex", func(t *testing.T) {
		rule := &kindav1.DbInstanceBackupRule{
			NamespaceRegex: "(abv",
			Name:           "test-backup",
			Cron:           "*/15 * * * *",
		}
		match, err := helpers.CheckBackupRules(rule, "test2-namespace")
		assert.Error(t, err)
		assert.False(t, match)
	})
}

func TestUnitCheckNamespaceFilter(t *testing.T) {
	t.Parallel()
	t.Run("CheckNamespaceFilter No matching filters", func(t *testing.T) {
		filters := []string{"test-.*"}
		match, err := helpers.CheckNamespaceFilter(filters, "other-namespace")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("CheckNamespaceFilter Matching filter", func(t *testing.T) {
		filters := []string{"test-.*"}
		match, err := helpers.CheckNamespaceFilter(filters, "test-namespace")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("CheckNamespaceFilter Invalid regex", func(t *testing.T) {
		filters := []string{"(abv"}
		match, err := helpers.CheckNamespaceFilter(filters, "test-namespace")
		assert.Error(t, err)
		assert.False(t, match)
	})
}

func TestUnitGetValueFromSource(t *testing.T) {
	namespace := "default"
	secretName := "my-secret"
	secretKey := "password"
	secretValue := "qwertyu9"

	configMapName := "my-config"
	configMapKey := "user"
	configMapValue := "db-operator"

	value := "direct-value"

	client := fake.NewClientBuilder().
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					secretKey: []byte(secretValue),
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					configMapKey: configMapValue,
				},
			},
		).
		Build()

	t.Parallel()
	t.Run("Value from is nil", func(t *testing.T) {
		result, err := helpers.GetValueFromSource(t.Context(), client, nil)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "valueFrom can't be nil")
		assert.Equal(t, "", result)
	})

	t.Run("Value from is empty", func(t *testing.T) {
		valueFrom := &kindav1.ValueSource{}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "valueSource must have either value or valueFrom")
		assert.Equal(t, "", result)
	})

	t.Run("Value is not nil", func(t *testing.T) {
		valueFrom := &kindav1.ValueSource{
			Value: &value,
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("Value is not nil but value from is empty", func(t *testing.T) {
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "valueFrom must have either secretRef or configMapRef")
		assert.Equal(t, "", result)
	})

	t.Run("Value from a secret that doesn't exist", func(t *testing.T) {
		fakeName := secretName + "-fake"
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				SecretKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &fakeName,
					Key:       &secretKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not found")
		assert.Equal(t, "", result)
	})

	t.Run("Value from a secret key that doesn't exist", func(t *testing.T) {
		fakeKey := secretKey + "-fake"
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				SecretKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &secretName,
					Key:       &fakeKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "key not found in secret")
		assert.Equal(t, "", result)
	})

	t.Run("Value from a secret key", func(t *testing.T) {
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				SecretKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &secretName,
					Key:       &secretKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.NoError(t, err)
		assert.Equal(t, secretValue, result)
	})

	t.Run("Value from a config map that doesn't exist", func(t *testing.T) {
		fakeName := configMapName + "-fake"
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				ConfigMapKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &fakeName,
					Key:       &configMapKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not found")
		assert.Equal(t, "", result)
	})

	t.Run("Value from a config map key that doesn't exist", func(t *testing.T) {
		fakeKey := configMapKey + "-fake"
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				ConfigMapKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &configMapName,
					Key:       &fakeKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "key not found in configmap")
		assert.Equal(t, "", result)
	})

	t.Run("Value from a config map key", func(t *testing.T) {
		valueFrom := &kindav1.ValueSource{
			ValueFrom: &kindav1.ValueFrom{
				ConfigMapKeyRef: &kindav1.SecretOrCMRef{
					Namespace: &namespace,
					Name:      &configMapName,
					Key:       &configMapKey,
				},
			},
		}
		result, err := helpers.GetValueFromSource(t.Context(), client, valueFrom)
		assert.NoError(t, err)
		assert.Equal(t, configMapValue, result)
	})
}

func TestUnitGetResourceFromValueSource(t *testing.T) {
	namespace := "default"
	secretName := "my-secret"
	secretKey := "password"
	secretValue := "qwertyu9"

	configMapName := "my-config"
	configMapKey := "user"
	configMapValue := "db-operator"

	client := fake.NewClientBuilder().
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					secretKey: []byte(secretValue),
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					configMapKey: configMapValue,
				},
			},
		).
		Build()

	t.Parallel()
	t.Run("Value from is nil", func(t *testing.T) {
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, nil)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "valueFrom can't be nil")
		assert.Equal(t, nil, result)
	})

	t.Run("Value from is empty", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "valueFrom must have either secretKeyRef or configMapKeyRef")
		assert.Equal(t, nil, result)
	})

	t.Run("Secret doesn't exist", func(t *testing.T) {
		fakeName := secretName + "-fake"
		valueFrom := &kindav1.ValueFrom{
			SecretKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Name:      &fakeName,
				Key:       &secretKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not found")
		assert.Equal(t, nil, result)
	})

	t.Run("Secret without namespace", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			SecretKeyRef: &kindav1.SecretOrCMRef{
				Name: &secretName,
				Key:  &secretKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "secretKeyRef must have both namespace and name")
		assert.Equal(t, nil, result)
	})

	t.Run("Secret without name", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			SecretKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Key:       &secretKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "secretKeyRef must have both namespace and name")
		assert.Equal(t, nil, result)
	})

	t.Run("Get a secret", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			SecretKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Name:      &secretName,
				Key:       &secretKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, secretName, result.GetName())
		_, ok := result.(*corev1.Secret)
		assert.True(t, ok)
	})

	t.Run("Config map doesn't exist", func(t *testing.T) {
		fakeName := configMapName + "-fake"
		valueFrom := &kindav1.ValueFrom{
			ConfigMapKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Name:      &fakeName,
				Key:       &configMapName,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "not found")
		assert.Equal(t, nil, result)
	})

	t.Run("Config map without namespace", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			ConfigMapKeyRef: &kindav1.SecretOrCMRef{
				Name: &configMapName,
				Key:  &configMapKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "configMapKeyRef must have both namespace and name")
		assert.Equal(t, nil, result)
	})

	t.Run("Config map without name", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			ConfigMapKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Key:       &configMapKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "configMapKeyRef must have both namespace and name")
		assert.Equal(t, nil, result)
	})

	t.Run("Get a config map", func(t *testing.T) {
		valueFrom := &kindav1.ValueFrom{
			ConfigMapKeyRef: &kindav1.SecretOrCMRef{
				Namespace: &namespace,
				Name:      &configMapName,
				Key:       &configMapKey,
			},
		}
		result, err := helpers.GetResourceFromValueSource(t.Context(), client, valueFrom)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, configMapName, result.GetName())
		_, ok := result.(*corev1.ConfigMap)
		assert.True(t, ok)
	})
}

func TestUnitObjectMetadataFormat(t *testing.T) {
	kind := "Secret"
	name := "credentials"
	namespace := "default"
	obj := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	assert.Equal(t, "Secret/default/credentials", helpers.ObjectMetadataFormat(obj))
}

func TestUnitObjectFromFormatdedString(t *testing.T) {
	t.Run("Misformatted string 1", func(t *testing.T) {
		entry := "simple string"
		result, err := helpers.ObjectFromFormattedString(entry)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid resource entry")
		assert.Nil(t, result)
	})
	t.Run("Misformatted string 3", func(t *testing.T) {
		entry := "secret/test/test/test"
		result, err := helpers.ObjectFromFormattedString(entry)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid resource entry")
		assert.Nil(t, result)
	})
	t.Run("Invalid type", func(t *testing.T) {
		entry := "Deployment/db-operator/db-operator"
		result, err := helpers.ObjectFromFormattedString(entry)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unsupported resource kind")
		assert.Nil(t, result)
	})
	t.Run("ConfigMap", func(t *testing.T) {
		entry := "ConfigMap/db-operator/db-operator"
		result, err := helpers.ObjectFromFormattedString(entry)
		assert.NoError(t, err)
		expected := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-operator",
				Namespace: "db-operator",
			},
		}
		assert.Equal(t, expected, result)
	})
	t.Run("Secret", func(t *testing.T) {
		entry := "Secret/db-operator/db-operator"
		result, err := helpers.ObjectFromFormattedString(entry)
		assert.NoError(t, err)
		expected := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-operator",
				Namespace: "db-operator",
			},
		}
		assert.Equal(t, expected, result)
	})
}
