/*
 * Copyright 2026 DB-Operator Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"testing"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnitDbUserDeleteCredentials(t *testing.T) {
	dbcr := &kindav1beta1.Database{
		Status: kindav1beta1.DatabaseStatus{
			DatabaseName: "test-database",
			Engine:       "postgres",
		},
	}

	t.Run("uses the username recorded in status", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			Status: kindav1beta1.DbUserStatus{UserName: "status-user"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "test-database", creds.Name)
		assert.Equal(t, "status-user", creds.Username)
	})

	t.Run("uses the Secret for a legacy DbUser", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{}
		secret := &corev1.Secret{Data: map[string][]byte{
			"POSTGRES_DB":       []byte("secret-database"),
			"POSTGRES_USER":     []byte("secret-user"),
			"POSTGRES_PASSWORD": []byte("password"),
		}}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, secret)
		require.NoError(t, err)
		assert.Equal(t, "test-database", creds.Name)
		assert.Equal(t, "secret-user", creds.Username)
	})

	t.Run("derives the username when the legacy Secret is missing", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "reader"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "test-reader", creds.Username)
	})

	t.Run("uses an existing username when the legacy Secret is missing", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "reader"},
			Spec:       kindav1beta1.DbUserSpec{ExistingUser: "existing-reader"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "existing-reader", creds.Username)
	})
}
