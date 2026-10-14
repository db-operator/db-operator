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

package database

import (
	"context"
	"database/sql"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitPostgresCleanupConnectionFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	p := &Postgres{Host: "127.0.0.1", Port: uint16(port), Database: "unreachable"}
	admin := &DatabaseUser{Username: "admin", Password: "unused"}
	user := &DatabaseUser{Username: "managed-user", AccessType: "readOnly"}
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{"database", func() error { return DeleteDatabase(t.Context(), p, admin) }},
		{"user", func() error { return DeleteUser(t.Context(), p, user, admin) }},
		{"existing user", func() error { return RevokePermissions(t.Context(), p, user, admin) }},
		{"drop role", func() error { return p.deleteUser(t.Context(), admin, user) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			require.Error(t, operation.run(), "a connection failure must not count as an absent object")
		})
	}
}

func TestPostgresCleanupQueryFailure(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	// NOLOGIN fails even on servers that use trust authentication.
	_, err := conn.ExecContext(t.Context(), "ALTER ROLE "+pq.QuoteIdentifier(user.Username)+" NOLOGIN")
	require.NoError(t, err)
	for _, operation := range []struct {
		name string
		run  func(context.Context, *DatabaseUser) error
	}{
		{"database", func(ctx context.Context, a *DatabaseUser) error { return DeleteDatabase(ctx, p, a) }},
		{"user", func(ctx context.Context, a *DatabaseUser) error { return DeleteUser(ctx, p, user, a) }},
		{"existing user", func(ctx context.Context, a *DatabaseUser) error { return RevokePermissions(ctx, p, user, a) }},
		{"drop role", func(ctx context.Context, a *DatabaseUser) error { return p.deleteUser(ctx, a, user) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			require.Error(t, operation.run(t.Context(), user))
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			require.ErrorIs(t, operation.run(ctx, admin), context.Canceled)
			assert.True(t, postgresCleanupRoleExists(t, conn, user.Username))
			assert.True(t, postgresCleanupDatabaseExists(t, conn, p.Database))
		})
	}
}

func TestPostgresCleanupAbsentRoleWithoutCreateRole(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	limitedAdmin := &DatabaseUser{Username: user.Username + "_admin", Password: "cleanup-password"}
	_, err := conn.ExecContext(t.Context(), "CREATE ROLE "+pq.QuoteIdentifier(limitedAdmin.Username)+
		" LOGIN NOSUPERUSER NOCREATEROLE PASSWORD "+pq.QuoteLiteral(limitedAdmin.Password))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec("DROP ROLE IF EXISTS " + pq.QuoteIdentifier(limitedAdmin.Username))
		assert.NoError(t, err)
	})
	limitedConn, err := p.getDbConn("postgres", limitedAdmin.Username, limitedAdmin.Password)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, limitedConn.Close()) })
	var createRole, superuser bool
	require.NoError(t, limitedConn.QueryRowContext(t.Context(),
		"SELECT rolcreaterole, rolsuper FROM pg_roles WHERE rolname = $1", limitedAdmin.Username).Scan(&createRole, &superuser))
	require.False(t, createRole)
	require.False(t, superuser)
	require.True(t, postgresCleanupRoleExists(t, limitedConn, user.Username))

	// A NOLOGIN role still exists, and a failed DROP must remain an error.
	_, err = conn.ExecContext(t.Context(), "ALTER ROLE "+pq.QuoteIdentifier(user.Username)+" NOLOGIN")
	require.NoError(t, err)
	err = p.deleteUser(t.Context(), limitedAdmin, user)
	var postgresError *pq.Error
	require.ErrorAs(t, err, &postgresError)
	assert.Equal(t, pq.ErrorCode("42501"), postgresError.Code)
	require.True(t, postgresCleanupRoleExists(t, conn, user.Username))

	require.NoError(t, p.deleteUser(t.Context(), admin, user))
	require.False(t, postgresCleanupRoleExists(t, limitedConn, user.Username))
	for _, accessType := range []string{ACCESS_TYPE_MAINUSER, "readOnly"} {
		t.Run(accessType, func(t *testing.T) {
			user.AccessType = accessType
			require.NoError(t, DeleteUser(t.Context(), p, user, limitedAdmin),
				"an absent role needs no DROP privilege")
		})
	}
}

func TestPostgresCleanupOrdinaryAndRepeatedDeletion(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
	require.False(t, postgresCleanupRoleExists(t, conn, user.Username))
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
	require.NoError(t, RevokePermissions(t.Context(), p, user, admin))
	require.NoError(t, DeleteDatabase(t.Context(), p, admin))
	require.False(t, postgresCleanupDatabaseExists(t, conn, p.Database))
	require.NoError(t, DeleteDatabase(t.Context(), p, admin))
}

func TestPostgresCleanupMainUserAfterDatabaseDeletion(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	user.AccessType = ACCESS_TYPE_MAINUSER
	require.NoError(t, DeleteDatabase(t.Context(), p, admin))
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
	assert.False(t, postgresCleanupDatabaseExists(t, conn, p.Database))
	assert.False(t, postgresCleanupRoleExists(t, conn, user.Username))
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
}

func TestPostgresCleanupMissingDatabase(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	extraRole := user.Username + "_extra"
	_, err := conn.ExecContext(t.Context(), "CREATE ROLE "+pq.QuoteIdentifier(extraRole))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec("DROP ROLE IF EXISTS " + pq.QuoteIdentifier(extraRole))
		assert.NoError(t, err)
	})
	_, err = conn.ExecContext(t.Context(), "GRANT "+pq.QuoteIdentifier(extraRole)+" TO "+pq.QuoteIdentifier(user.Username))
	require.NoError(t, err)
	user.ExtraPrivileges = []string{extraRole}
	_, err = conn.ExecContext(t.Context(), "DROP DATABASE "+pq.QuoteIdentifier(p.Database))
	require.NoError(t, err)

	require.NoError(t, RevokePermissions(t.Context(), p, user, admin))
	require.True(t, postgresCleanupRoleExists(t, conn, user.Username))
	var membership bool
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT pg_has_role($1, $2, 'MEMBER')", user.Username, extraRole).Scan(&membership))
	assert.False(t, membership, "cluster-wide membership must be revoked even when the database is gone")
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
	assert.False(t, postgresCleanupRoleExists(t, conn, user.Username))
	require.NoError(t, DeleteDatabase(t.Context(), p, admin))
}

func TestPostgresCleanupDependentRole(t *testing.T) {
	p, user, admin, conn := postgresCleanupFixture(t)
	externalName := p.Database + "_external"
	_, err := conn.ExecContext(t.Context(), "CREATE DATABASE "+pq.QuoteIdentifier(externalName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(externalName))
		assert.NoError(t, err)
	})
	external, err := p.getDbConn(externalName, admin.Username, admin.Password)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, external.Close()) })
	_, err = external.ExecContext(t.Context(), "CREATE TABLE retained (id integer)")
	require.NoError(t, err)
	_, err = external.ExecContext(t.Context(), "ALTER TABLE retained OWNER TO "+pq.QuoteIdentifier(user.Username))
	require.NoError(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		err = DeleteUser(t.Context(), p, user, admin)
		var postgresError *pq.Error
		require.ErrorAs(t, err, &postgresError)
		assert.Equal(t, pq.ErrorCode("2BP01"), postgresError.Code)
		assert.True(t, postgresCleanupRoleExists(t, conn, user.Username))
		var tableExists bool
		require.NoError(t, external.QueryRowContext(t.Context(), "SELECT to_regclass('public.retained') IS NOT NULL").Scan(&tableExists))
		assert.True(t, tableExists, "cleanup must not drop dependencies in another database")
	}
	_, err = external.ExecContext(t.Context(), "DROP TABLE retained")
	require.NoError(t, err)
	require.NoError(t, DeleteUser(t.Context(), p, user, admin))
	assert.False(t, postgresCleanupRoleExists(t, conn, user.Username))
	assert.True(t, postgresCleanupDatabaseExists(t, conn, externalName))
}

func TestPostgresCleanupExistingRole(t *testing.T) {
	for _, login := range []string{"LOGIN", "NOLOGIN"} {
		t.Run(login, func(t *testing.T) {
			p, user, admin, conn := postgresCleanupFixture(t)
			_, err := conn.ExecContext(t.Context(), "ALTER ROLE "+pq.QuoteIdentifier(user.Username)+" "+login)
			require.NoError(t, err)
			target, err := p.getDbConn(p.Database, admin.Username, admin.Password)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, target.Close()) })
			_, err = target.ExecContext(t.Context(), "CREATE TABLE granted (id integer)")
			require.NoError(t, err)
			_, err = target.ExecContext(t.Context(), "GRANT SELECT ON granted TO "+pq.QuoteIdentifier(user.Username))
			require.NoError(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				require.NoError(t, RevokePermissions(t.Context(), p, user, admin))
				assert.True(t, postgresCleanupRoleExists(t, conn, user.Username))
				var hasSelect bool
				require.NoError(t, target.QueryRowContext(t.Context(), "SELECT has_table_privilege($1, 'public.granted', 'SELECT')", user.Username).Scan(&hasSelect))
				assert.False(t, hasSelect)
			}
		})
	}
}

func postgresCleanupFixture(t *testing.T) (*Postgres, *DatabaseUser, *DatabaseUser, *sql.DB) {
	t.Helper()
	p, user := testPostgres()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	p.Database = "cleanup_" + unique
	user.Username = "cleanup_user_" + unique
	user.AccessType = "readOnly"
	admin := getPostgresAdmin()
	conn, err := p.getDbConn("postgres", admin.Username, admin.Password)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })
	require.NoError(t, conn.PingContext(t.Context()))
	t.Cleanup(func() {
		_, err := conn.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(p.Database))
		assert.NoError(t, err)
		_, err = conn.Exec("DROP ROLE IF EXISTS " + pq.QuoteIdentifier(user.Username))
		assert.NoError(t, err)
	})
	_, err = conn.ExecContext(t.Context(), "CREATE DATABASE "+pq.QuoteIdentifier(p.Database))
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), "CREATE ROLE "+pq.QuoteIdentifier(user.Username)+" LOGIN")
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), "GRANT "+pq.QuoteIdentifier(user.Username)+" TO "+pq.QuoteIdentifier(admin.Username))
	require.NoError(t, err)
	return p, user, admin, conn
}

func postgresCleanupRoleExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", name).Scan(&exists))
	return exists
}

func postgresCleanupDatabaseExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists))
	return exists
}
