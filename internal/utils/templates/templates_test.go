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

package templates_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/utils/templates"
	consts "github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/utils/database"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var secretK8s *corev1.Secret = &corev1.Secret{
	ObjectMeta: v1.ObjectMeta{
		Name: "creds",
	},
	Data: map[string][]byte{
		"POSTGRES_PASSWORD": []byte("testpassword"),
		"POSTGRES_USER":     []byte("testuser"),
	},
}

var secretK8sUser *corev1.Secret = &corev1.Secret{
	ObjectMeta: v1.ObjectMeta{
		Name: "creds-user",
	},
	Data: map[string][]byte{
		"POSTGRES_PASSWORD": []byte("testpassword"),
		"POSTGRES_USER":     []byte("testuser"),
	},
}

var configmapK8s *corev1.ConfigMap = &corev1.ConfigMap{
	ObjectMeta: v1.ObjectMeta{
		Name:        "creds",
		Annotations: map[string]string{},
	},
	Data: map[string]string{
		"SSL_MODE": "required",
	},
}

var databaseK8s *v1beta1.Database = &v1beta1.Database{
	TypeMeta: v1.TypeMeta{
		Kind: "Database",
	},
	ObjectMeta: v1.ObjectMeta{
		Name:      "database",
		Namespace: "default",
	},
	Spec: v1beta1.DatabaseSpec{
		SecretName: "creds",
	},
}

var dbuserK8s *v1beta1.DbUser = &v1beta1.DbUser{
	TypeMeta: v1.TypeMeta{
		Kind: "DbUser",
	},
	ObjectMeta: v1.ObjectMeta{
		Name:      "dbuser",
		Namespace: "default",
	},
	Spec: v1beta1.DbUserSpec{
		SecretName: "creds-user",
	},
}

var secretPostgres *corev1.Secret = &corev1.Secret{
	ObjectMeta: v1.ObjectMeta{
		Name:        "creds",
		Annotations: map[string]string{},
	},
	Data: map[string][]byte{
		consts.POSTGRES_USER:     []byte("testusername"),
		consts.POSTGRES_PASSWORD: []byte("testpassword"),
		consts.POSTGRES_DB:       []byte("database"),
	},
}

var secretMysql *corev1.Secret = &corev1.Secret{
	ObjectMeta: v1.ObjectMeta{
		Name:        "creds",
		Annotations: map[string]string{},
	},
	Data: map[string][]byte{
		consts.MYSQL_USER:     []byte("testusername"),
		consts.MYSQL_PASSWORD: []byte("testpassword"),
		consts.MYSQL_DB:       []byte("database"),
	},
}

var db database.Database = database.New("dummy")

func TestUnitTemplatedCredsDS(t *testing.T) {
	t.Parallel()
	t.Run("New template DS for database", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, &templates.TemplateDataSources{
			DatabaseK8sObj:    databaseK8s,
			DbUserK8sObj:      nil,
			SecretK8sObj:      secretK8s,
			ConfigMapK8sObj:   configmapK8s,
			DatabaseObj:       db,
			DatabaseUser:      nil,
			ExtraTemplateVars: map[string]string{},
		}, templateds)
	})

	t.Run("New template DS for dbuser", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, secretK8sUser, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, &templates.TemplateDataSources{
			DatabaseK8sObj:    databaseK8s,
			DbUserK8sObj:      dbuserK8s,
			SecretK8sObj:      secretK8sUser,
			ConfigMapK8sObj:   configmapK8s,
			DatabaseObj:       db,
			DatabaseUser:      nil,
			ExtraTemplateVars: map[string]string{},
		}, templateds)
	})

	t.Run("Secret doesn't belong to a database", func(t *testing.T) {
		newSecret := secretK8s.DeepCopy()
		newSecret.Name = "newname"
		_, err := templates.NewTemplateDataSource(databaseK8s, nil, newSecret, configmapK8s, db, nil, nil)
		assert.Error(t, errors.New("secret newname doesn't belong to the database database"), err)
	})

	t.Run("Secret doesn't belong to a user", func(t *testing.T) {
		newSecret := secretK8s.DeepCopy()
		newSecret.Name = "creds"
		_, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, newSecret, configmapK8s, db, nil, nil)
		assert.Error(t, errors.New("secret creds doesn't belong to the DbUser dbuser"), err)
	})

	t.Run("Secret is nil", func(t *testing.T) {
		_, err := templates.NewTemplateDataSource(databaseK8s, nil, nil, configmapK8s, db, nil, nil)
		assert.Error(t, errors.New("secret must be passed"), err)
	})

	t.Run("ConfigMap doesn't belong to a database", func(t *testing.T) {
		newConfigmap := configmapK8s.DeepCopy()
		newConfigmap.Name = "newname"
		_, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, newConfigmap, db, nil, nil)
		assert.Error(t, errors.New("configmap newname doesn't belong to the database database"), err)
	})

	t.Run("ConfigMap doesn't belong to a user", func(t *testing.T) {
		newConfigMap := configmapK8s.DeepCopy()
		newConfigMap.Name = "newname"
		_, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, secretK8s, newConfigMap, db, nil, nil)
		assert.Error(t, errors.New("confugnap newname doesn't belong to the DbUser dbuser"), err)
	})

	t.Run("ConfigMap is nil", func(t *testing.T) {
		_, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, nil, db, nil, nil)
		assert.Error(t, errors.New("configmap must be passed"), err)
	})

	t.Run("Database is nil", func(t *testing.T) {
		_, err := templates.NewTemplateDataSource(nil, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.Error(t, errors.New("database must be passed"), err)
	})
}

func TestUnitTemplatedCredsConfigMap(t *testing.T) {
	t.Parallel()
	t.Run("Fetch data from a configmap", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		entry, err := templateds.ConfigMap("SSL_MODE")
		assert.NoError(t, err)
		t.Logf("entry: %v", configmapK8s.Data)
		assert.Equal(t, "required", entry)
	})

	t.Run("Non-existent configmap key", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.ConfigMap("SOMETHING")
		assert.Error(t, errors.New("entry not found in the configmap: SOMETHING"), err)
	})
}

func TestUnitTemplatedCredsInstanceVars(t *testing.T) {
	t.Parallel()
	t.Run("Fetch data from instance vars", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, map[string]string{"test": "test"})
		assert.NoError(t, err)
		entry, err := templateds.InstanceVar("test")
		assert.NoError(t, err)
		assert.Equal(t, "test", entry)
	})

	t.Run("Non-existent instance var", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, map[string]string{"test": "test"})
		assert.NoError(t, err)
		entry, err := templateds.InstanceVar("testNotExist")
		assert.Empty(t, entry)
		assert.Error(t, errors.New("variable is not found"), err)
	})
}

func TestUnitTemplatedCredsSecret(t *testing.T) {
	t.Parallel()
	t.Run("Fetch data from a secret", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		entry, err := templateds.Secret("POSTGRES_PASSWORD")
		assert.NoError(t, err)
		assert.Equal(t, "testpassword", entry)
	})

	t.Run("Non-existent secret key", func(t *testing.T) {
		templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.Secret("SOMETHING")
		assert.Error(t, errors.New("entry not found in the secret: SOMETHING"), err)
	})
}

func TestUnitTemplatesProtocol(t *testing.T) {
	t.Run("Get postgres protocol", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		proto, err := templateds.Protocol()
		assert.NoError(t, err)
		assert.Equal(t, "postgresql", proto)
	})

	t.Run("Get mysql protocol", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_MYSQL
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		proto, err := templateds.Protocol()
		assert.NoError(t, err)
		assert.Equal(t, "mysql", proto)
	})

	t.Run("Unknown engine while fetching protocol", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = "dymmysql"
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.Protocol()
		assert.ErrorContains(t, err, "unknown engine")
	})
}

func TestUnitTemplatesUsername(t *testing.T) {
	t.Run("Get postgres username", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		username, err := templateds.Username()
		assert.NoError(t, err)
		assert.Equal(t, "testusername", username)
	})

	t.Run("Get mysql username", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_MYSQL
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		username, err := templateds.Username()
		assert.NoError(t, err)
		assert.Equal(t, "testusername", username)
	})

	t.Run("Unknown engine while fetching username", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = "dymmysql"
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.Username()
		assert.ErrorContains(t, err, "unknown engine")
	})
}

func TestUnitTemplatesPassword(t *testing.T) {
	t.Run("Get postgres password", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		password, err := templateds.Password()
		assert.NoError(t, err)
		assert.Equal(t, "testpassword", password)
	})

	t.Run("Get mysql password", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_MYSQL
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		password, err := templateds.Password()
		assert.NoError(t, err)
		assert.Equal(t, "testpassword", password)
	})

	t.Run("Unknown engine while fetching password", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = "dymmysql"
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.Password()
		assert.ErrorContains(t, err, "unknown engine")
	})
}

func TestUnitTemplatesDatabase(t *testing.T) {
	t.Run("Get postgres database", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		password, err := templateds.Database()
		assert.NoError(t, err)
		assert.Equal(t, "database", password)
	})
	t.Run("Get mysql database", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_MYSQL
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		password, err := templateds.Database()
		assert.NoError(t, err)
		assert.Equal(t, "database", password)
	})

	t.Run("Unknown engine while fetching database", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = "dymmysql"
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		_, err = templateds.Database()
		assert.ErrorContains(t, err, "unknown engine")
	})
}

func TestUnitTemplatesHost(t *testing.T) {
	t.Run("Get host without proxy", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		hostname, err := templateds.Hostname()
		assert.NoError(t, err)
		assert.Equal(t, database.DB_DUMMY_HOSTNAME, hostname)
	})

	t.Run("Get host with proxy", func(t *testing.T) {
		expecterHostname := "proxy-hostname"
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.ProxyStatus.Status = true
		databaseNew.Status.ProxyStatus.ServiceName = expecterHostname
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		hostname, err := templateds.Hostname()
		assert.NoError(t, err)
		assert.Equal(t, expecterHostname, hostname)
	})
}

func TestUnitTemplatesPort(t *testing.T) {
	t.Run("Get host with proxy", func(t *testing.T) {
		var expectedPort int32 = 1122
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.ProxyStatus.Status = true
		databaseNew.Status.ProxyStatus.SQLPort = expectedPort
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		hostname, err := templateds.Port()
		assert.NoError(t, err)
		assert.Equal(t, expectedPort, hostname)
	})
	t.Run("Get host without proxy", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil, nil)
		assert.NoError(t, err)
		port, err := templateds.Port()
		assert.NoError(t, err)
		assert.Equal(t, int32(database.DB_DUMMY_PORT), port)
	})
}

func TestUnitTemplatesRender(t *testing.T) {
	t.Run("HTML templates", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres.DeepCopy(), configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"), nil)
		assert.NoError(t, err)
		expectedResult := []byte("<div>")
		err = templateds.Render(v1beta1.Templates{
			&v1beta1.Template{
				Name:     "HTML_TEST",
				Template: "<div>",
				Secret:   true,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, templateds.SecretK8sObj.Data["HTML_TEST"])
	})

	t.Run("Full secret template", func(t *testing.T) {
		expectedResult := map[string][]byte{
			"STRING":         []byte("STRING"),
			"PASSWORD_EXTRA": []byte("testpassword"),
			"REUSE_PREVIOUS": []byte("STRING"),
			"SEC_PASSWORD":   []byte("testpassword"),
			"GO_FUNCTION":    []byte("It's true"),
		}
		maps.Copy(expectedResult, secretPostgres.Data)
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres.DeepCopy(), configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"), nil)
		assert.NoError(t, err)
		err = templateds.Render(v1beta1.Templates{
			&v1beta1.Template{
				Name:     "STRING",
				Template: "STRING",
				Secret:   true,
			},
			&v1beta1.Template{
				Name:     "PASSWORD_EXTRA",
				Template: "{{ .Password }}",
				Secret:   true,
			},
			&v1beta1.Template{
				Name:     "REUSE_PREVIOUS",
				Template: "{{ .Secret \"STRING\" }}",
				Secret:   true,
			},
			&v1beta1.Template{
				Name:     "SEC_PASSWORD",
				Template: "{{ .Secret \"POSTGRES_PASSWORD\" }}",
				Secret:   true,
			},
			&v1beta1.Template{
				Name:     "GO_FUNCTION",
				Template: "{{ if eq 1 1 }}It's true{{ else }}It's false{{ end }}",
				Secret:   true,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, templateds.SecretK8sObj.Data)
	})

	t.Run("Full cm template", func(t *testing.T) {
		expectedResult := map[string]string{
			"STRING":         "STRING",
			"PASSWORD_EXTRA": "testpassword",
			"REUSE_PREVIOUS": "STRING",
			"SSL_MODE_AGAIN": configmapK8s.Data["SSL_MODE"],
		}

		maps.Copy(expectedResult, configmapK8s.Data)
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres.DeepCopy(), configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"), nil)
		assert.NoError(t, err)
		err = templateds.Render(v1beta1.Templates{
			&v1beta1.Template{
				Name:     "STRING",
				Template: "STRING",
				Secret:   false,
			},
			&v1beta1.Template{
				Name:     "PASSWORD_EXTRA",
				Template: "{{ .Password }}",
				Secret:   false,
			},
			&v1beta1.Template{
				Name:     "REUSE_PREVIOUS",
				Template: "{{ .ConfigMap \"STRING\" }}",
				Secret:   false,
			},
			&v1beta1.Template{
				Name:     "SSL_MODE_AGAIN",
				Template: "{{ .ConfigMap \"SSL_MODE\" }}",
				Secret:   false,
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, templateds.ConfigMapK8sObj.Data)
	})

	t.Run("Blocked fields errors", func(t *testing.T) {
		databaseNew := databaseK8s.DeepCopy()
		databaseNew.Status.Engine = consts.ENGINE_POSTGRES
		templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"), nil)
		assert.NoError(t, err)
		blockedFields := []string{"POSTGRES_DB", "POSTGRES_PASSWORD", "POSTGRES_USER", "DB", "USER", "PASSWORD"}
		for _, name := range blockedFields {
			err := templateds.Render(v1beta1.Templates{
				&v1beta1.Template{
					Name:     name,
					Template: "DUMMY",
					Secret:   false,
				},
			})
			assert.ErrorContains(t, err, "not allowed for templating", name)
		}
	})
}
