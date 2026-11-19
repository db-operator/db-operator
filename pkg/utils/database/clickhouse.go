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
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/db-operator/db-operator/v2/pkg/utils/kci"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Clickhouse struct {
	Host         string
	Port         uint16
	Database     string
	Cluster      string
	SSLEnabled   bool
	SkipCAVerify bool
}

const clickhouseDefaultAdmin = "default"

func clickhouseQuoteIdent(name string) string {
	return "`" + strings.NewReplacer("\\", "\\\\", "`", "\\`").Replace(name) + "`"
}

func clickhouseQuoteString(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(value) + "'"
}

func (c Clickhouse) onCluster() string {
	if c.Cluster == "" {
		return ""
	}
	return " ON CLUSTER " + clickhouseQuoteIdent(c.Cluster)
}

func (c Clickhouse) getDbConn(ctx context.Context, user, password string) (*sql.DB, error) {
	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", c.Host, c.Port)},
		Auth: clickhouse.Auth{
			Username: user,
			Password: password,
		},
		DialTimeout: 10 * time.Second,
	}
	if c.SSLEnabled {
		opts.TLS = &tls.Config{InsecureSkipVerify: c.SkipCAVerify}
	}

	db := clickhouse.OpenDB(opts)
	db.SetMaxIdleConns(0)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (c Clickhouse) executeQuery(ctx context.Context, query string, admin *DatabaseUser) error {
	log := log.FromContext(ctx)
	db, err := c.getDbConn(ctx, admin.Username, admin.Password)
	if err != nil {
		log.Error(err, "failed to get db connection")
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, query); err != nil {
		log.Error(err, "failed to execute a query")
		return err
	}

	return nil
}

func (c Clickhouse) execAsUser(ctx context.Context, query string, user *DatabaseUser) error {
	return c.executeQuery(ctx, query, user)
}

func (c Clickhouse) isUserExist(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) bool {
	log := log.FromContext(ctx)
	db, err := c.getDbConn(ctx, admin.Username, admin.Password)
	if err != nil {
		log.Error(err, "failed to get db connection")
		return false
	}
	defer db.Close()

	var count uint64
	if err := db.QueryRowContext(ctx, "SELECT count() FROM system.users WHERE name = ?", user.Username).Scan(&count); err != nil {
		log.Error(err, "failed to check if a user exists")
		return false
	}

	return count > 0
}

func (c Clickhouse) CheckStatus(ctx context.Context, user *DatabaseUser) error {
	db, err := c.getDbConn(ctx, user.Username, user.Password)
	if err != nil {
		return fmt.Errorf("db conn test failed - could not establish a connection: %w", err)
	}
	defer db.Close()

	var count uint64
	if err := db.QueryRowContext(ctx, "SELECT count() FROM system.databases WHERE name = ?", c.Database).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("database %s doesn't exist or isn't accessible by %s", c.Database, user.Username)
	}

	return nil
}

func (c Clickhouse) GetCredentials(ctx context.Context, user *DatabaseUser) Credentials {
	return Credentials{
		Name:     c.Database,
		Username: user.Username,
		Password: user.Password,
	}
}

func (c Clickhouse) ParseAdminCredentials(ctx context.Context, data map[string][]byte) (*DatabaseUser, error) {
	admin := &DatabaseUser{Username: clickhouseDefaultAdmin}

	if user, ok := data["user"]; ok {
		admin.Username = string(user)
	}

	if password, ok := data["password"]; ok {
		admin.Password = string(password)
		return admin, nil
	}

	return admin, errors.New("can not find clickhouse admin credentials")
}

func (c Clickhouse) GetDatabaseAddress(ctx context.Context) DatabaseAddress {
	return DatabaseAddress{
		Host: c.Host,
		Port: c.Port,
	}
}

func (c Clickhouse) QueryAsUser(ctx context.Context, query string, user *DatabaseUser) (string, error) {
	log := log.FromContext(ctx)
	db, err := c.getDbConn(ctx, user.Username, user.Password)
	if err != nil {
		log.Error(err, "failed to get db connection")
		return "", err
	}
	defer db.Close()

	var result string
	if err := db.QueryRowContext(ctx, query).Scan(&result); err != nil {
		log.Error(err, "an error occurred while executing a query")
		return "", err
	}

	return result, nil
}

func (c Clickhouse) createDatabase(ctx context.Context, admin *DatabaseUser) error {
	create := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s%s", clickhouseQuoteIdent(c.Database), c.onCluster())
	if c.Cluster != "" {
		create += fmt.Sprintf(
			" ENGINE = Replicated(%s, '{shard}', '{replica}')",
			clickhouseQuoteString("/clickhouse/databases/"+c.Database),
		)
	}

	return c.executeQuery(ctx, create, admin)
}

func (c Clickhouse) deleteDatabase(ctx context.Context, admin *DatabaseUser) error {
	log := log.FromContext(ctx)
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %s%s SYNC", clickhouseQuoteIdent(c.Database), c.onCluster())

	return kci.Retry(3, 5*time.Second, func() error {
		if err := c.executeQuery(ctx, drop, admin); err != nil {
			log.V(2).Info("failed with error, retrying", "error", err)
			return err
		}
		return nil
	})
}

func (c Clickhouse) createOrUpdateUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	if !c.isUserExist(ctx, admin, user) {
		return c.createUser(ctx, admin, user)
	}

	return c.updateUser(ctx, admin, user)
}

func (c Clickhouse) createUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	if c.isUserExist(ctx, admin, user) {
		return fmt.Errorf("user already exists: %s", user.Username)
	}

	create := fmt.Sprintf(
		"CREATE USER %s%s IDENTIFIED WITH sha256_password BY %s",
		clickhouseQuoteIdent(user.Username), c.onCluster(), clickhouseQuoteString(user.Password),
	)
	if err := c.executeQuery(ctx, create, admin); err != nil {
		return err
	}

	return c.setUserPermission(ctx, admin, user)
}

func (c Clickhouse) updateUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	if !c.isUserExist(ctx, admin, user) {
		return fmt.Errorf("user doesn't exist yet: %s", user.Username)
	}

	update := fmt.Sprintf(
		"ALTER USER %s%s IDENTIFIED WITH sha256_password BY %s",
		clickhouseQuoteIdent(user.Username), c.onCluster(), clickhouseQuoteString(user.Password),
	)
	if err := c.executeQuery(ctx, update, admin); err != nil {
		return err
	}

	return c.setUserPermission(ctx, admin, user)
}

func (c Clickhouse) setUserPermission(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	var privileges string
	switch user.AccessType {
	case ACCESS_TYPE_MAINUSER:
		privileges = "ALL"
	case ACCESS_TYPE_READONLY:
		privileges = "SELECT"
	case ACCESS_TYPE_READWRITE:
		privileges = "SELECT, INSERT, ALTER UPDATE, ALTER DELETE"
	default:
		return fmt.Errorf("unknown access type: %s", user.AccessType)
	}

	grant := fmt.Sprintf(
		"GRANT%s %s ON %s.* TO %s",
		c.onCluster(), privileges, clickhouseQuoteIdent(c.Database), clickhouseQuoteIdent(user.Username),
	)
	if err := c.executeQuery(ctx, grant, admin); err != nil {
		return err
	}

	for _, role := range user.ExtraPrivileges {
		grantRole := fmt.Sprintf("GRANT%s %s TO %s", c.onCluster(), clickhouseQuoteIdent(role), clickhouseQuoteIdent(user.Username))
		if err := c.executeQuery(ctx, grantRole, admin); err != nil {
			return err
		}
	}

	return nil
}

func (c Clickhouse) revokePermissions(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	if !c.isUserExist(ctx, admin, user) {
		return nil
	}

	for _, role := range user.ExtraPrivileges {
		revokeRole := fmt.Sprintf("REVOKE%s %s FROM %s", c.onCluster(), clickhouseQuoteIdent(role), clickhouseQuoteIdent(user.Username))
		if err := c.executeQuery(ctx, revokeRole, admin); err != nil {
			return err
		}
	}

	revoke := fmt.Sprintf(
		"REVOKE%s ALL ON %s.* FROM %s",
		c.onCluster(), clickhouseQuoteIdent(c.Database), clickhouseQuoteIdent(user.Username),
	)

	return c.executeQuery(ctx, revoke, admin)
}

func (c Clickhouse) deleteUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	drop := fmt.Sprintf("DROP USER IF EXISTS %s%s", clickhouseQuoteIdent(user.Username), c.onCluster())

	return c.executeQuery(ctx, drop, admin)
}

func (c Clickhouse) GetServerVersion(ctx context.Context, user *DatabaseUser) (string, error) {
	db, err := c.getDbConn(ctx, user.Username, user.Password)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return "", errors.New("failed to get server version")
	}

	return version, nil
}

func (c Clickhouse) listNames(ctx context.Context, user *DatabaseUser, query string) ([]string, error) {
	db, err := c.getDbConn(ctx, user.Username, user.Password)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	return names, rows.Err()
}

func (c Clickhouse) ListDatabases(ctx context.Context, user *DatabaseUser) ([]string, error) {
	return c.listNames(ctx, user, "SELECT name FROM system.databases ORDER BY name")
}

func (c Clickhouse) ListUsers(ctx context.Context, user *DatabaseUser) ([]string, error) {
	return c.listNames(ctx, user, "SELECT name FROM system.users ORDER BY name")
}
