/*
 * Copyright 2024 Datacosmos
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

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// Register the "clickhouse" sql driver
	_ "github.com/ClickHouse/clickhouse-go"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClickHouse is a database interface, abstracted object
// represents a database on ClickHouse instance
// can be used to execute queries to ClickHouse database
type ClickHouse struct {
	Host        string
	Port        uint16
	Database    string
	ClusterName string
}

// Internal helpers, these functions are not part for the `Database` interface

func (ch ClickHouse) getDbConn(dbname, user, password string) (*sql.DB, error) {
	dataSourceName := fmt.Sprintf("tcp://%s:%d?database=%s&username=%s&password=%s", ch.Host, ch.Port, dbname, user, password)
	db, err := sql.Open("clickhouse", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %v", err)
	}

	return db, err
}

func (ch ClickHouse) executeExec(ctx context.Context, database, query string, admin *DatabaseUser) error {
	log := log.FromContext(ctx)
	db, err := ch.getDbConn(database, admin.Username, admin.Password)
	if err != nil {
		log.Error(err, "failed to open a db connection")
		return err
	}

	defer db.Close()
	_, err = db.Exec(query)

	return err
}

func (ch ClickHouse) isDbExist(ctx context.Context, admin *DatabaseUser) bool {
	check := fmt.Sprintf("SELECT name FROM system.databases WHERE name = '%s'", ch.Database)

	return ch.isRowExist(ctx, "default", check, admin.Username, admin.Password)
}

func (ch ClickHouse) isUserExist(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) bool {
	check := fmt.Sprintf("SELECT name FROM system.users WHERE name = '%s'", user.Username)

	return ch.isRowExist(ctx, "default", check, admin.Username, admin.Password)
}

func (ch ClickHouse) isRowExist(ctx context.Context, database, query, user, password string) bool {
	log := log.FromContext(ctx)
	db, err := ch.getDbConn(database, user, password)
	if err != nil {
		log.Error(err, "failed to open a db connection")
		return false
	}
	defer db.Close()

	var name string
	err = db.QueryRow(query).Scan(&name)
	if err != nil {
		log.V(2).Info("failed executing query", "error", err)
		return false
	}
	return true
}

// Functions that implement the `Database` interface

// CheckStatus checks status of ClickHouse database
// if the connection to database works
func (ch ClickHouse) CheckStatus(ctx context.Context, user *DatabaseUser) error {
	db, err := ch.getDbConn(ch.Database, user.Username, user.Password)
	if err != nil {
		return fmt.Errorf("db conn test failed - couldn't get db conn: %s", err)
	}
	defer db.Close()
	res, err := db.Query("SELECT 1")
	if err != nil {
		return fmt.Errorf("db conn test failed - failed to execute query: %s", err)
	}
	res.Close()

	return nil
}

// GetCredentials returns credentials of the ClickHouse database
func (ch ClickHouse) GetCredentials(ctx context.Context, user *DatabaseUser) Credentials {
	return Credentials{
		Name:     ch.Database,
		Username: user.Username,
		Password: user.Password,
	}
}

// ParseAdminCredentials parse admin username and password of ClickHouse database from secret data
func (ch ClickHouse) ParseAdminCredentials(ctx context.Context, data map[string][]byte) (*DatabaseUser, error) {
	admin := &DatabaseUser{}

	if user, ok := data["user"]; ok {
		admin.Username = string(user)
	} else {
		return nil, errors.New("no admin user found")
	}

	if password, ok := data["password"]; ok {
		admin.Password = string(password)
	} else {
		return nil, errors.New("no admin password found")
	}

	return admin, nil
}

func (ch ClickHouse) GetDatabaseAddress(ctx context.Context) DatabaseAddress {
	return DatabaseAddress{
		Host: ch.Host,
		Port: ch.Port,
	}
}

func (ch ClickHouse) QueryAsUser(ctx context.Context, query string, user *DatabaseUser) (string, error) {
	log := log.FromContext(ctx)
	db, err := ch.getDbConn(ch.Database, user.Username, user.Password)
	if err != nil {
		log.Error(err, "failed to open a db connection")
		return "", err
	}
	defer db.Close()

	var result string
	if err := db.QueryRow(query).Scan(&result); err != nil {
		log.Error(err, "failed executing query", "query", query)
		return "", err
	}
	return result, nil
}

func (ch ClickHouse) createDatabase(ctx context.Context, admin *DatabaseUser) error {
	log := log.FromContext(ctx)
	var create string

	if ch.ClusterName != "" {
		create = fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` ON CLUSTER '%s'", ch.Database, ch.ClusterName)
	} else {
		create = fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", ch.Database)
	}

	if !ch.isDbExist(ctx, admin) {
		err := ch.executeExec(ctx, "default", create, admin)
		if err != nil {
			log.Error(err, "failed creating ClickHouse database")
			return err
		}
	}

	return nil
}

func (ch ClickHouse) deleteDatabase(ctx context.Context, admin *DatabaseUser) error {
	log := log.FromContext(ctx)
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", ch.Database)

	if ch.isDbExist(ctx, admin) {
		err := ch.executeExec(ctx, "default", drop, admin)
		if err != nil {
			log.Error(err, "failed dropping ClickHouse database")
			return err
		}
	}
	return nil
}

func (ch ClickHouse) createOrUpdateUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	if !ch.isUserExist(ctx, admin, user) {
		if err := ch.createUser(ctx, admin, user); err != nil {
			log.Error(err, "failed creating ClickHouse user")
			return err
		}
	} else {
		if err := ch.updateUser(ctx, admin, user); err != nil {
			log.Error(err, "failed updating ClickHouse user")
			return err
		}
	}

	if err := ch.setUserPermission(ctx, admin, user); err != nil {
		return err
	}
	return nil
}

func (ch ClickHouse) createUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	create := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s' IDENTIFIED BY '%s'", user.Username, user.Password)

	if ch.ClusterName != "" {
		create += fmt.Sprintf(" ON CLUSTER '%s'", ch.ClusterName)
	}

	if err := ch.executeExec(ctx, "default", create, admin); err != nil {
		log.Error(err, "failed creating ClickHouse user")
		return err
	}

	return nil
}

func (ch ClickHouse) updateUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	update := fmt.Sprintf("ALTER USER '%s' IDENTIFIED BY '%s'", user.Username, user.Password)

	if ch.ClusterName != "" {
		update += fmt.Sprintf(" ON CLUSTER '%s'", ch.ClusterName)
	}

	if err := ch.executeExec(ctx, "default", update, admin); err != nil {
		log.Error(err, "failed updating ClickHouse user")
		return err
	}

	return nil
}

func (ch ClickHouse) setUserPermission(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	grant := fmt.Sprintf("GRANT ALL ON `%s`.* TO '%s'", ch.Database, user.Username)

	if ch.ClusterName != "" {
		grant += fmt.Sprintf(" ON CLUSTER '%s'", ch.ClusterName)
	}

	if err := ch.executeExec(ctx, ch.Database, grant, admin); err != nil {
		log.Error(err, "failed granting privileges to ClickHouse user")
		return err
	}

	return nil
}

func (ch ClickHouse) revokePermissions(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	if !ch.isUserExist(ctx, admin, user) {
		return nil
	}

	revoke := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM '%s'", ch.Database, user.Username)

	if ch.ClusterName != "" {
		revoke += fmt.Sprintf(" ON CLUSTER '%s'", ch.ClusterName)
	}

	if err := ch.executeExec(ctx, ch.Database, revoke, admin); err != nil {
		log.Error(err, "failed revoking privileges from ClickHouse user")
		return err
	}

	return nil
}

func (ch ClickHouse) execAsUser(ctx context.Context, query string, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	db, err := ch.getDbConn(ch.Database, user.Username, user.Password)
	if err != nil {
		log.Error(err, "failed to open a db connection")
		return err
	}

	defer db.Close()
	_, err = db.Exec(query)

	return err
}

func (ch ClickHouse) deleteUser(ctx context.Context, admin *DatabaseUser, user *DatabaseUser) error {
	log := log.FromContext(ctx)
	drop := fmt.Sprintf("DROP USER IF EXISTS '%s'", user.Username)

	if ch.ClusterName != "" {
		drop += fmt.Sprintf(" ON CLUSTER '%s'", ch.ClusterName)
	}

	if err := ch.executeExec(ctx, "default", drop, admin); err != nil {
		log.Error(err, "failed deleting ClickHouse user")
		return err
	}

	return nil
}
