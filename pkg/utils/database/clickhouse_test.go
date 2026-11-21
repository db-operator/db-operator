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

package database

import (
	"context"
	"testing"

	"github.com/db-operator/db-operator/v2/pkg/test"
	"github.com/stretchr/testify/assert"
)

func testClickhouse() (*Clickhouse, *DatabaseUser) {
	return &Clickhouse{
			Host:     test.GetClickhouseHost(),
			Port:     test.GetClickhousePort(),
			Database: "testdb",
			Cluster:  test.GetClickhouseCluster(),
		}, &DatabaseUser{
			Username:   "testuser",
			Password:   "testpwd",
			AccessType: ACCESS_TYPE_MAINUSER,
		}
}

func getClickhouseAdmin() *DatabaseUser {
	return &DatabaseUser{
		Username: test.GetClickhouseAdminUser(),
		Password: test.GetClickhouseAdminPassword(),
	}
}

func clickhouseDatabaseEngine(t *testing.T, c *Clickhouse, admin *DatabaseUser) string {
	db, err := c.getDbConn(context.TODO(), admin.Username, admin.Password)
	assert.NoError(t, err)
	defer db.Close()

	var engine string
	err = db.QueryRow("SELECT engine FROM system.databases WHERE name = ?", c.Database).Scan(&engine)
	if err != nil {
		return ""
	}
	return engine
}

func TestClickhouseCheckStatus(t *testing.T) {
	c, dbu := testClickhouse()
	admin := getClickhouseAdmin()
	assert.Error(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
	assert.Error(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.Error(t, c.CheckStatus(context.TODO(), dbu))
}

func TestClickhouseExecuteQuery(t *testing.T) {
	c, _ := testClickhouse()
	admin := getClickhouseAdmin()
	assert.NoError(t, c.executeQuery(context.TODO(), "SELECT 1", admin))

	admin.Password = "wrongpass"
	assert.Error(t, c.executeQuery(context.TODO(), "SELECT 1", admin))
}

func TestClickhouseCreateDatabase(t *testing.T) {
	admin := getClickhouseAdmin()
	c, _ := testClickhouse()
	c.Database = "createtest"

	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createDatabase(context.TODO(), admin))

	expected := "Atomic"
	if c.Cluster != "" {
		expected = "Replicated"
	}
	assert.Equal(t, expected, clickhouseDatabaseEngine(t, c, admin))

	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseCreateDatabaseWithoutCluster(t *testing.T) {
	admin := getClickhouseAdmin()
	c, _ := testClickhouse()
	c.Database = "createtest-local"
	c.Cluster = ""

	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.Equal(t, "Atomic", clickhouseDatabaseEngine(t, c, admin))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
	assert.Equal(t, "", clickhouseDatabaseEngine(t, c, admin))
}

func TestClickhouseCreateOrUpdateUser(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()

	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	assert.True(t, c.isUserExist(context.TODO(), admin, dbu))

	dbu.Password = "newpwd"
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseQueryAsUser(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))

	assert.NoError(t, c.execAsUser(context.TODO(), "CREATE TABLE testdb.test (id UInt32, name String) ENGINE = MergeTree ORDER BY id", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "INSERT INTO testdb.test VALUES (1, 'test')", dbu))

	res, err := c.QueryAsUser(context.TODO(), "SELECT name FROM testdb.test", dbu)
	assert.NoError(t, err)
	assert.Equal(t, "test", res)

	_, err = c.QueryAsUser(context.TODO(), "SELECT * FROM testdb.test", dbu)
	assert.Error(t, err)

	assert.NoError(t, c.execAsUser(context.TODO(), "DROP TABLE testdb.test SYNC", dbu))
	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseMainUserLifecycle(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	c.Database = "maintest"
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))

	assert.NoError(t, c.execAsUser(context.TODO(), "CREATE TABLE maintest.test_1 (id UInt32, name String) ENGINE = MergeTree ORDER BY id", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "INSERT INTO maintest.test_1 VALUES (1, 'test-1')", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "SELECT * FROM maintest.test_1", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "ALTER TABLE maintest.test_1 UPDATE name = 'test-1-new' WHERE id = 1", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "ALTER TABLE maintest.test_1 DELETE WHERE id = 1", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "ALTER TABLE maintest.test_1 ADD COLUMN extra String", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "DROP TABLE maintest.test_1 SYNC", dbu))

	assert.Error(t, c.execAsUser(context.TODO(), "CREATE DATABASE othertest", dbu))
	assert.Error(t, c.execAsUser(context.TODO(), "CREATE USER otheruser", dbu))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseReadOnlyUserLifecycle(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	c.Database = "readonlytest"
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	readonlyUser := &DatabaseUser{
		Username:   "readonly",
		Password:   "123123",
		AccessType: ACCESS_TYPE_READONLY,
	}

	assert.NoError(t, c.execAsUser(context.TODO(), "CREATE TABLE readonlytest.test_1 (id UInt32, name String) ENGINE = MergeTree ORDER BY id", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "INSERT INTO readonlytest.test_1 VALUES (1, 'test-1')", dbu))

	assert.NoError(t, c.createUser(context.TODO(), admin, readonlyUser))
	assert.Error(t, c.createUser(context.TODO(), admin, readonlyUser))
	assert.NoError(t, c.updateUser(context.TODO(), admin, readonlyUser))

	assert.NoError(t, c.execAsUser(context.TODO(), "SELECT * FROM readonlytest.test_1", readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "INSERT INTO readonlytest.test_1 VALUES (2, 'test-2')", readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "ALTER TABLE readonlytest.test_1 UPDATE name = 'x' WHERE id = 1", readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "ALTER TABLE readonlytest.test_1 DELETE WHERE id = 1", readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "CREATE TABLE readonlytest.test_2 (id UInt32) ENGINE = MergeTree ORDER BY id", readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "DROP TABLE readonlytest.test_1", readonlyUser))

	assert.NoError(t, c.execAsUser(context.TODO(), "CREATE TABLE readonlytest.test_2 (id UInt32) ENGINE = MergeTree ORDER BY id", dbu))
	assert.NoError(t, c.execAsUser(context.TODO(), "SELECT * FROM readonlytest.test_2", readonlyUser))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, readonlyUser))
	assert.False(t, c.isUserExist(context.TODO(), admin, readonlyUser))
	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseReadWriteUserLifecycle(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	c.Database = "readwritetest"
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	readwriteUser := &DatabaseUser{
		Username:   "readwrite",
		Password:   "123123",
		AccessType: ACCESS_TYPE_READWRITE,
	}

	assert.NoError(t, c.execAsUser(context.TODO(), "CREATE TABLE readwritetest.test_1 (id UInt32, name String) ENGINE = MergeTree ORDER BY id", dbu))
	assert.NoError(t, c.createUser(context.TODO(), admin, readwriteUser))

	assert.NoError(t, c.execAsUser(context.TODO(), "INSERT INTO readwritetest.test_1 VALUES (1, 'test-1')", readwriteUser))
	assert.NoError(t, c.execAsUser(context.TODO(), "SELECT * FROM readwritetest.test_1", readwriteUser))
	assert.NoError(t, c.execAsUser(context.TODO(), "ALTER TABLE readwritetest.test_1 UPDATE name = 'x' WHERE id = 1", readwriteUser))
	assert.NoError(t, c.execAsUser(context.TODO(), "ALTER TABLE readwritetest.test_1 DELETE WHERE id = 1", readwriteUser))
	assert.Error(t, c.execAsUser(context.TODO(), "CREATE TABLE readwritetest.test_2 (id UInt32) ENGINE = MergeTree ORDER BY id", readwriteUser))
	assert.Error(t, c.execAsUser(context.TODO(), "DROP TABLE readwritetest.test_1", readwriteUser))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, readwriteUser))
	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseRevokePermissions(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	c.Database = "revoketest"
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.revokePermissions(context.TODO(), admin, dbu))
	assert.Error(t, c.CheckStatus(context.TODO(), dbu))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.revokePermissions(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseExtraPrivileges(t *testing.T) {
	admin := getClickhouseAdmin()
	c, _ := testClickhouse()
	c.Database = "extraprivilegestest"
	shared := &Clickhouse{Host: c.Host, Port: c.Port, Database: "extraprivilegesshared", Cluster: c.Cluster}
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, shared.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.executeQuery(context.TODO(), "CREATE TABLE extraprivilegesshared.test_1 (id UInt32) ENGINE = MergeTree ORDER BY id", admin))
	assert.NoError(t, c.executeQuery(context.TODO(), "CREATE ROLE IF NOT EXISTS extra_reader"+c.onCluster(), admin))
	assert.NoError(t, c.executeQuery(context.TODO(), "GRANT"+c.onCluster()+" SELECT ON extraprivilegesshared.* TO extra_reader", admin))

	readonlyUser := &DatabaseUser{
		Username:        "readonlyextra",
		Password:        "123123",
		AccessType:      ACCESS_TYPE_READONLY,
		ExtraPrivileges: []string{"extra_reader"},
	}

	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, readonlyUser))
	assert.NoError(t, c.execAsUser(context.TODO(), "SELECT * FROM extraprivilegesshared.test_1", readonlyUser))

	assert.NoError(t, c.revokePermissions(context.TODO(), admin, readonlyUser))
	assert.Error(t, c.execAsUser(context.TODO(), "SELECT * FROM extraprivilegesshared.test_1", readonlyUser))
	assert.Error(t, c.CheckStatus(context.TODO(), readonlyUser))

	assert.NoError(t, c.deleteUser(context.TODO(), admin, readonlyUser))
	assert.NoError(t, c.executeQuery(context.TODO(), "DROP ROLE IF EXISTS extra_reader"+c.onCluster(), admin))
	assert.NoError(t, shared.deleteDatabase(context.TODO(), admin))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseDeleteUser(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.False(t, c.isUserExist(context.TODO(), admin, dbu))
}

func TestClickhouseDeleteDatabase(t *testing.T) {
	admin := getClickhouseAdmin()
	c, _ := testClickhouse()

	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
	assert.Equal(t, "", clickhouseDatabaseEngine(t, c, admin))
}

func TestClickhouseListDatabasesAndUsers(t *testing.T) {
	admin := getClickhouseAdmin()
	c, dbu := testClickhouse()
	c.Database = "listtest"
	assert.NoError(t, c.createDatabase(context.TODO(), admin))
	assert.NoError(t, c.createOrUpdateUser(context.TODO(), admin, dbu))

	databases, err := c.ListDatabases(context.TODO(), admin)
	assert.NoError(t, err)
	assert.Contains(t, databases, "listtest")

	users, err := c.ListUsers(context.TODO(), admin)
	assert.NoError(t, err)
	assert.Contains(t, users, dbu.Username)

	assert.NoError(t, c.deleteUser(context.TODO(), admin, dbu))
	assert.NoError(t, c.deleteDatabase(context.TODO(), admin))
}

func TestClickhouseGetCredentials(t *testing.T) {
	c, dbu := testClickhouse()

	cred := c.GetCredentials(context.TODO(), dbu)
	assert.Equal(t, dbu.Username, cred.Username)
	assert.Equal(t, c.Database, cred.Name)
	assert.Equal(t, dbu.Password, cred.Password)
}

func TestClickhouseParseAdminCredentials(t *testing.T) {
	c, _ := testClickhouse()

	_, err := c.ParseAdminCredentials(context.TODO(), map[string][]byte{"unknownkey": []byte("wrong")})
	assert.Error(t, err)

	cred, err := c.ParseAdminCredentials(context.TODO(), map[string][]byte{"user": []byte("admin"), "password": []byte("secret")})
	assert.NoError(t, err)
	assert.Equal(t, "admin", cred.Username)
	assert.Equal(t, "secret", cred.Password)

	cred, err = c.ParseAdminCredentials(context.TODO(), map[string][]byte{"password": []byte("secret")})
	assert.NoError(t, err)
	assert.Equal(t, "default", cred.Username)
}

func TestClickhouseGetServerVersion(t *testing.T) {
	admin := getClickhouseAdmin()
	c, _ := testClickhouse()
	got, err := c.GetServerVersion(t.Context(), admin)
	assert.NoError(t, err)
	assert.NotEmpty(t, got)
}

func TestClickhouseQuoting(t *testing.T) {
	assert.Equal(t, "`ns-db`", clickhouseQuoteIdent("ns-db"))
	assert.Equal(t, "`a\\`b`", clickhouseQuoteIdent("a`b"))
	assert.Equal(t, "'it\\'s'", clickhouseQuoteString("it's"))
	assert.Equal(t, "'a\\\\b'", clickhouseQuoteString("a\\b"))
}
