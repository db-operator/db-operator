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
	"testing"

	"github.com/db-operator/db-operator/v2/api/v1beta2"
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

var databaseK8s *v1beta2.Database = &v1beta2.Database{
	TypeMeta: v1.TypeMeta{
		Kind: "Database",
	},
	ObjectMeta: v1.ObjectMeta{
		Name:      "database",
		Namespace: "default",
	},
	Spec: v1beta2.DatabaseSpec{
		Credentials: v1beta2.Credentials{
			SecretName: "creds",
		},
	},
}

var dbuserK8s *v1beta2.DbUser = &v1beta2.DbUser{
	TypeMeta: v1.TypeMeta{
		Kind: "DbUser",
	},
	ObjectMeta: v1.ObjectMeta{
		Name:      "dbuser",
		Namespace: "default",
	},
	Spec: v1beta2.DbUserSpec{
		Credentials: v1beta2.Credentials{
			SecretName: "creds-user",
		},
	},
}

var db database.Database = database.New("dummy")

func TestUnitNewDSDatabase(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	assert.NoError(t, err)
	assert.Equal(t, &templates.TemplateDataSources{
		DatabaseK8sObj:  databaseK8s,
		DbUserK8sObj:    nil,
		SecretK8sObj:    secretK8s,
		ConfigMapK8sObj: configmapK8s,
		DatabaseObj:     db,
		DatabaseUser:    nil,
	}, templateds)
}

func TestUnitNewDSDatabaseUser(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, secretK8sUser, configmapK8s, db, nil)
	assert.NoError(t, err)
	assert.Equal(t, &templates.TemplateDataSources{
		DatabaseK8sObj:  databaseK8s,
		DbUserK8sObj:    dbuserK8s,
		SecretK8sObj:    secretK8sUser,
		ConfigMapK8sObj: configmapK8s,
		DatabaseObj:     db,
		DatabaseUser:    nil,
	}, templateds)
}

func TestUnitNewDSSecretOwnershipError(t *testing.T) {
	newSecret := secretK8s.DeepCopy()
	newSecret.ObjectMeta.Name = "newname"
	_, err := templates.NewTemplateDataSource(databaseK8s, nil, newSecret, configmapK8s, db, nil)
	assert.Error(t, errors.New("secret newname doesn't belong to the database database"), err)
}

func TestUnitNewDSSecretOwnershipUserError(t *testing.T) {
	newSecret := secretK8s.DeepCopy()
	newSecret.ObjectMeta.Name = "creds"
	_, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, newSecret, configmapK8s, db, nil)
	assert.Error(t, errors.New("secret creds doesn't belong to the DbUser dbuser"), err)
}

func TestUnitNewDSSecretNotPassedError(t *testing.T) {
	_, err := templates.NewTemplateDataSource(databaseK8s, nil, nil, configmapK8s, db, nil)
	assert.Error(t, errors.New("secret must be passed"), err)
}

func TestUnitNewDSConfigMapOwnershipError(t *testing.T) {
	newConfigmap := configmapK8s.DeepCopy()
	newConfigmap.ObjectMeta.Name = "newname"
	_, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, newConfigmap, db, nil)
	assert.Error(t, errors.New("configmap newname doesn't belong to the database database"), err)
}

func TestUnitNewDSConfigMapOwnershipUserNoError(t *testing.T) {
	_, err := templates.NewTemplateDataSource(databaseK8s, dbuserK8s, secretK8sUser, configmapK8s, db, nil)
	assert.NoError(t, err)
}

func TestUnitNewDSConfigMapNotPassedError(t *testing.T) {
	_, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, nil, db, nil)
	assert.Error(t, errors.New("configmap must be passed"), err)
}

func TestUnitNewDSDatabaseNotPassedError(t *testing.T) {
	_, err := templates.NewTemplateDataSource(nil, nil, secretK8s, configmapK8s, db, nil)
	assert.Error(t, errors.New("database must be passed"), err)
}

func TestUnitTemplatesSecret(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	entry, err := templateds.Secret("POSTGRES_PASSWORD")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "testpassword", entry)
}

func TestUnitTemplatesSecretErr(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	_, err = templateds.Secret("SOMETHING")
	assert.Error(t, errors.New("entry not found in the secret: SOMETHING"), err)
}

func TestUnitTemplatesConfigMap(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	entry, err := templateds.ConfigMap("SSL_MODE")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "required", entry)
}

func TestUnitTemplatesQueryErr(t *testing.T) {
	err := errors.New("something went south")
	var dbNew database.Database = database.Dummy{
		Error: err,
	}

	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, dbNew, nil)
	if err != nil {
		t.Error(err)
	}
	_, err = templateds.Query("dummy")
	assert.ErrorContains(t, err, err.Error())
}

func TestUnitTemplatesQuery(t *testing.T) {
	query := "SELECT SOMETHING FROM SOMETHING"
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	result, err := templateds.Query(query)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, query, result)
}

func TestUnitTemplatesConfigMapErr(t *testing.T) {
	templateds, err := templates.NewTemplateDataSource(databaseK8s, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	_, err = templateds.ConfigMap("SOMETHING")
	assert.Error(t, errors.New("entry not found in the configmap: SOMETHING"), err)
}

func TestUnitProtocolGetterPostgres(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	proto, err := templateds.Protocol()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "postgresql", proto)
}

func TestUnitProtocolGetterMysql(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_MYSQL
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	proto, err := templateds.Protocol()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "mysql", proto)
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

func TestUnitUsernameGetterPostgres(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	username, err := templateds.Username()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "testusername", username)
}

func TestUnitUsernameGetterMysql(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_MYSQL
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	username, err := templateds.Username()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "testusername", username)
}

func TestUnitUsernameGetterUnknownEngineError(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = "dymmysql"
	_, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil)
	assert.Error(t, errors.New("unknown engine: fake"), err)
}

func TestUnitPasswordGetterPostgres(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	password, err := templateds.Password()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "testpassword", password)
}

func TestUnitPasswordGetterMysql(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_MYSQL
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	password, err := templateds.Password()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "testpassword", password)
}

func TestUnitPasswordGetterUnknownEngineError(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = "dymmysql"
	_, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil)
	assert.Error(t, errors.New("unknown engine: fake"), err)
}

func TestUnitDatabaseGetterPostgres(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	password, err := templateds.Database()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "database", password)
}

func TestUnitDatabaseGetterMysql(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_MYSQL
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	password, err := templateds.Database()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "database", password)
}

func TestUnitDatabaseGetterUnknownEngineError(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = "dymmysql"
	_, err := templates.NewTemplateDataSource(databaseNew, nil, secretK8s, configmapK8s, db, nil)
	assert.Error(t, errors.New("unknown engine: fake"), err)
}

func TestUnitHostGetterNoProxy(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	hostname, err := templateds.Hostname()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, database.DB_DUMMY_HOSTNAME, hostname)
}

func TestUnitPortGetterNoProxy(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretMysql, configmapK8s, db, nil)
	if err != nil {
		t.Error(err)
	}
	port, err := templateds.Port()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, int32(database.DB_DUMMY_PORT), port)
}

func TestUnitRenderErrDupSecret(t *testing.T) {
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres, configmapK8s, db, database.NewDummyUser("mainUser"))
	if err != nil {
		t.Error(err)
	}
	err = templateds.Render(v1beta2.Templates{
		&v1beta2.Template{
			Name:     "POSTGRES_PASSWORD",
			Template: "DUMMY",
		},
	})
	assert.ErrorContains(t, err, "POSTGRES_PASSWORD already exists in the secret")
}

func TestUnitRenderAppendCustomSecret(t *testing.T) {
	expectedResult := map[string][]byte{
		"STRING":         []byte("STRING"),
		"PASSWORD":       []byte("testpassword"),
		"REUSE_PREVIOUS": []byte("STRING"),
		"SEC_PASSWORD":   []byte("testpassword"),
		"GO_FUNCTION":    []byte("It's true"),
	}
	for key, val := range secretPostgres.Data {
		expectedResult[key] = val
	}
	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretPostgres.DeepCopy(), configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"))
	if err != nil {
		t.Error(err)
	}
	if err := templateds.Render(v1beta2.Templates{
		&v1beta2.Template{
			Name:     "STRING",
			Template: "STRING",
		},
		&v1beta2.Template{
			Name:     "PASSWORD",
			Template: "{{ .Password }}",
		},
		&v1beta2.Template{
			Name:     "REUSE_PREVIOUS",
			Template: "{{ .Secret \"STRING\" }}",
		},
		&v1beta2.Template{
			Name:     "SEC_PASSWORD",
			Template: "{{ .Secret \"POSTGRES_PASSWORD\" }}",
		},
		&v1beta2.Template{
			Name:     "GO_FUNCTION",
			Template: "{{ if eq 1 1 }}It's true{{ else }}It's false{{ end }}",
		},
	}); err != nil {
		t.Error(err)
	}
	assert.Equal(t, expectedResult, templateds.SecretK8sObj.Data)
	assert.Equal(t, "STRING,PASSWORD,REUSE_PREVIOUS,SEC_PASSWORD,GO_FUNCTION",
		templateds.SecretK8sObj.ObjectMeta.Annotations[consts.TEMPLATE_ANNOTATION_KEY],
	)
}

func TestUnitRenderCleanupSecret(t *testing.T) {
	expectedResult := map[string][]byte{
		"PASSWORD": []byte("testpassword"),
	}
	secretNew := secretPostgres.DeepCopy()
	secretNew.Data["PRESERVED"] = []byte("PRESERVED")
	for key, val := range secretNew.Data {
		expectedResult[key] = val
	}

	databaseNew := databaseK8s.DeepCopy()
	databaseNew.Status.Engine = consts.ENGINE_POSTGRES
	templateds, err := templates.NewTemplateDataSource(databaseNew, nil, secretNew, configmapK8s.DeepCopy(), db, database.NewDummyUser("mainUser"))
	if err != nil {
		t.Error(err)
	}
	if err := templateds.Render(v1beta2.Templates{
		&v1beta2.Template{
			Name:     "STRING",
			Template: "STRING",
		},
		&v1beta2.Template{
			Name:     "PASSWORD",
			Template: "{{ .Secret \"POSTGRES_PASSWORD\" }}",
		},
	}); err != nil {
		t.Error(err)
	}
	if err := templateds.Render(v1beta2.Templates{
		&v1beta2.Template{
			Name:     "PASSWORD",
			Template: "{{ .Secret \"POSTGRES_PASSWORD\" }}",
		},
	}); err != nil {
		t.Error(err)
	}
	assert.Equal(t, expectedResult, templateds.SecretK8sObj.Data)
	assert.Equal(t, "PASSWORD",
		templateds.SecretK8sObj.ObjectMeta.Annotations[consts.TEMPLATE_ANNOTATION_KEY],
	)
}
