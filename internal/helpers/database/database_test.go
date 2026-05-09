/*
 * Copyright 2021 kloeckner.i GmbH
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

package database_test

import (
	"context"
	"testing"

	kindav1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	dbhelper "github.com/db-operator/db-operator/v2/internal/helpers/database"
	"github.com/db-operator/db-operator/v2/internal/utils/testutils"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/utils/database"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testDbcred = database.Credentials{Name: "testdb", Username: "testuser", Password: "password"}
	ctx        = context.Background()
)

func TestUnitDeterminPostgresType(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	postgresDbCr := testutils.NewPostgresTestDbCr(instance)
	db, _, _ := dbhelper.FetchDatabaseData(ctx, postgresDbCr, testDbcred, &instance)
	_, ok := db.(database.Postgres)
	assert.Equal(t, ok, true, "expected true")
}

func TestUnitDeterminMysqlType(t *testing.T) {
	mysqlDbCr := testutils.NewMysqlTestDbCr()
	instance := testutils.NewPostgresTestDbInstanceCr()
	db, _, _ := dbhelper.FetchDatabaseData(ctx, mysqlDbCr, testDbcred, &instance)
	_, ok := db.(database.Mysql)
	assert.Equal(t, ok, true, "expected true")
}

func TestUnitParsePostgresSecretData(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	postgresDbCr := testutils.NewPostgresTestDbCr(instance)

	invalidData := make(map[string][]byte)
	invalidData["DB"] = []byte("testdb")

	_, err := dbhelper.ParseDatabaseSecretData(postgresDbCr, invalidData)
	assert.Errorf(t, err, "should get error %v", err)

	validData := make(map[string][]byte)
	validData["POSTGRES_DB"] = []byte("testdb")
	validData["POSTGRES_USER"] = []byte("testuser")
	validData["POSTGRES_PASSWORD"] = []byte("testpassword")

	cred, err := dbhelper.ParseDatabaseSecretData(postgresDbCr, validData)
	assert.NoErrorf(t, err, "expected no error %v", err)
	assert.Equal(t, string(validData["POSTGRES_DB"]), cred.Name, "expect same values")
	assert.Equal(t, string(validData["POSTGRES_USER"]), cred.Username, "expect same values")
	assert.Equal(t, string(validData["POSTGRES_PASSWORD"]), cred.Password, "expect same values")
}

func TestUnitParseMysqlSecretData(t *testing.T) {
	mysqlDbCr := testutils.NewMysqlTestDbCr()

	invalidData := make(map[string][]byte)
	invalidData["DB"] = []byte("testdb")

	_, err := dbhelper.ParseDatabaseSecretData(mysqlDbCr, invalidData)
	assert.Errorf(t, err, "should get error %v", err)

	validData := make(map[string][]byte)
	validData["DB"] = []byte("testdb")
	validData["USER"] = []byte("testuser")
	validData["PASSWORD"] = []byte("testpassword")

	cred, err := dbhelper.ParseDatabaseSecretData(mysqlDbCr, validData)
	assert.NoErrorf(t, err, "expected no error %v", err)
	assert.Equal(t, string(validData["DB"]), cred.Name, "expect same values")
	assert.Equal(t, string(validData["USER"]), cred.Username, "expect same values")
	assert.Equal(t, string(validData["PASSWORD"]), cred.Password, "expect same values")
}

func TestUnitGenerateDatabaseSecretData(t *testing.T) {
	objMeta := metav1.ObjectMeta{Namespace: "team-a", Name: "app-db"}

	postgresData, err := dbhelper.GenerateDatabaseSecretData(objMeta, "postgres", "")
	assert.NoError(t, err)
	assert.Equal(t, "team-a-app-db", string(postgresData[consts.POSTGRES_DB]))
	assert.Equal(t, "team-a-app-db", string(postgresData[consts.POSTGRES_USER]))
	assert.NotEmpty(t, postgresData[consts.POSTGRES_PASSWORD])

	mysqlData, err := dbhelper.GenerateDatabaseSecretData(objMeta, "mysql", "")
	assert.NoError(t, err)
	assert.Equal(t, "team_a_app_db", string(mysqlData[consts.MYSQL_DB]))
	assert.Equal(t, "team_a_app_db", string(mysqlData[consts.MYSQL_USER]))
	assert.NotEmpty(t, mysqlData[consts.MYSQL_PASSWORD])

	_, err = dbhelper.GenerateDatabaseSecretData(objMeta, "oracle", "")
	assert.Error(t, err)
}

func TestUnitMonitoringNotEnabled(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	instance.Spec.Monitoring.Enabled = false
	postgresDbCr := testutils.NewPostgresTestDbCr(instance)
	db, _, _ := dbhelper.FetchDatabaseData(ctx, postgresDbCr, testDbcred, &instance)
	postgresInterface, _ := db.(database.Postgres)

	found := false
	for _, ext := range postgresInterface.Extensions {
		if ext == "pg_stat_statements" {
			found = true
			break
		}
	}
	assert.Equal(t, found, false, "expected pg_stat_statement is not included in extension list")
}

func TestUnitMonitoringEnabled(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	instance.Spec.Monitoring.Enabled = true
	postgresDbCr := testutils.NewPostgresTestDbCr(instance)

	db, _, _ := dbhelper.FetchDatabaseData(ctx, postgresDbCr, testDbcred, &instance)
	postgresInterface, _ := db.(database.Postgres)

	assert.Equal(t, postgresInterface.Monitoring, true, "expected monitoring is true in postgres interface")
}

func TestUnitGetGenericSSLModePostgres(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	posgresDbCR := testutils.NewPostgresTestDbCr(instance)
	instance.Spec.SSLConnection.Enabled = false
	instance.Spec.SSLConnection.SkipVerify = false
	mode, err := dbhelper.GetGenericSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_DISABLED, mode)

	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetGenericSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_DISABLED, mode)

	instance.Spec.SSLConnection.Enabled = true
	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetGenericSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_REQUIRED, mode)

	instance.Spec.SSLConnection.SkipVerify = false
	mode, err = dbhelper.GetGenericSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_VERIFY_CA, mode)
}

func TestUnitGetGenericSSLModeMysql(t *testing.T) {
	mysqlDbCR := testutils.NewMysqlTestDbCr()
	instance := testutils.NewMysqlTestDbInstanceCr()

	instance.Spec.SSLConnection.Enabled = false
	instance.Spec.SSLConnection.SkipVerify = false
	mode, err := dbhelper.GetGenericSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_DISABLED, mode)

	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetGenericSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_DISABLED, mode)

	instance.Spec.SSLConnection.Enabled = true
	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetGenericSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_REQUIRED, mode)

	instance.Spec.SSLConnection.SkipVerify = false
	mode, err = dbhelper.GetGenericSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, consts.SSL_VERIFY_CA, mode)
}

func TestUnitGetSSLModePostgres(t *testing.T) {
	instance := testutils.NewPostgresTestDbInstanceCr()
	posgresDbCR := testutils.NewPostgresTestDbCr(instance)

	instance.Spec.SSLConnection.Enabled = false
	instance.Spec.SSLConnection.SkipVerify = false
	mode, err := dbhelper.GetSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "disable", mode)

	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "disable", mode)

	instance.Spec.SSLConnection.Enabled = true
	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "require", mode)

	instance.Spec.SSLConnection.SkipVerify = false
	mode, err = dbhelper.GetSSLMode(posgresDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "verify-ca", mode)
}

func TestUnitGetSSLModeMysql(t *testing.T) {
	mysqlDbCR := testutils.NewMysqlTestDbCr()
	instance := testutils.NewMysqlTestDbInstanceCr()

	instance.Spec.SSLConnection.Enabled = false
	instance.Spec.SSLConnection.SkipVerify = false
	mode, err := dbhelper.GetSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "disabled", mode)

	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "disabled", mode)

	instance.Spec.SSLConnection.Enabled = true
	instance.Spec.SSLConnection.SkipVerify = true
	mode, err = dbhelper.GetSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "required", mode)

	instance.Spec.SSLConnection.SkipVerify = false
	mode, err = dbhelper.GetSSLMode(mysqlDbCR, &instance)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "verify_ca", mode)
}

func TestUnitGetSSLModeUnknownEngine(t *testing.T) {
	dbCR := &kindav1beta2.Database{Status: kindav1beta2.DatabaseStatus{Engine: "oracle"}}
	instance := testutils.NewMysqlTestDbInstanceCr()

	_, err := dbhelper.GetSSLMode(dbCR, &instance)
	assert.ErrorContains(t, err, "unknown database engine")
}

func TestUnitFetchDatabaseDataUnknownEngine(t *testing.T) {
	dbCR := &kindav1beta2.Database{Status: kindav1beta2.DatabaseStatus{Engine: "oracle"}}
	instance := testutils.NewPostgresTestDbInstanceCr()

	db, user, err := dbhelper.FetchDatabaseData(ctx, dbCR, testDbcred, &instance)
	assert.ErrorContains(t, err, "not supported engine type")
	assert.Nil(t, db)
	assert.Nil(t, user)
}
