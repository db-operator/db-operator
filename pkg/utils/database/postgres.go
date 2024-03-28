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

package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	// Don't delete below package. Used for driver "cloudsqlpostgres"
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	"github.com/db-operator/db-operator/pkg/utils/kci"
	"github.com/go-logr/logr"

	// Don't delete below package. Used for driver "postgres"
	"github.com/lib/pq"
)

// Postgres is a database interface, abstraced object
// represents a database on postgres instance
// can be used to execute query to postgres database
type Postgres struct {
	Backend          string
	Host             string
	Port             uint16
	Database         string
	Monitoring       bool
	Extensions       []string
	SSLEnabled       bool
	SkipCAVerify     bool
	DropPublicSchema bool
	Schemas          []string
	Template         string
	// A user that is created with the Database
	//  it's required to set default priveleges
	//  for additional users
	MainUser *DatabaseUser
	Log      logr.Logger
}

const postgresDefaultSSLMode = "disable"

// Internal helpers, these functions are not part for the `Database` interface

func (p Postgres) sslMode() string {
	if !p.SSLEnabled {
		return "disable"
	}

	if p.SSLEnabled && !p.SkipCAVerify {
		return "verify-ca"
	}

	if p.SSLEnabled && p.SkipCAVerify {
		return "require"
	}

	return postgresDefaultSSLMode
}

func (p Postgres) getDbConn(dbname, user, password string) (*sql.DB, error) {
	var db *sql.DB
	var sqldriver string

	switch p.Backend {
	case "google":
		sqldriver = "cloudsqlpostgres"
	default:
		sqldriver = "postgres"
	}

	dataSourceName := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s", p.Host, p.Port, dbname, user, password, p.sslMode())
	db, err := sql.Open(sqldriver, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %v", err)
	}

	return db, err
}

func (p Postgres) executeExec(database, query string, admin *DatabaseUser) error {
	db, err := p.getDbConn(database, admin.Username, admin.Password)
	if err != nil {
		p.Log.Error(err, "failed to open a db connection")
		return err
	}

	defer db.Close()
	_, err = db.Exec(query)

	return err
}

func (p Postgres) execAsUser(query string, user *DatabaseUser) error {
	db, err := p.getDbConn(p.Database, user.Username, user.Password)
	if err != nil {
		p.Log.Error(err, "failed to open a db connection")
		return err
	}

	defer db.Close()
	_, err = db.Exec(query)

	return err
}

func (p Postgres) isDbExist(admin *DatabaseUser) bool {
	check := fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s';", p.Database)

	return p.isRowExist("postgres", check, admin.Username, admin.Password)
}

func (p Postgres) isUserExist(admin *DatabaseUser, user *DatabaseUser) bool {
	check := fmt.Sprintf("SELECT 1 FROM pg_user WHERE usename = '%s';", user.Username)

	return p.isRowExist("postgres", check, admin.Username, admin.Password)
}

func (p Postgres) isRowExist(database, query, user, password string) bool {
	db, err := p.getDbConn(database, user, password)
	if err != nil {
		p.Log.Error(err, "failed to open a db connection")
		return false
	}
	defer db.Close()

	var name string
	err = db.QueryRow(query).Scan(&name)
	if err != nil {
		p.Log.V(2).Info("failed executing query", "error", err)
		return false
	}
	return true
}

func (p Postgres) dropPublicSchema(admin *DatabaseUser) error {
	if p.Monitoring {
		return fmt.Errorf("can not drop public schema when monitoring is enabled on instance level")
	}

	drop := "DROP SCHEMA IF EXISTS public;"
	if err := p.executeExec(p.Database, drop, admin); err != nil {
		p.Log.Error(err, "failed to drop the schema Public")
		return err
	}
	return nil
}

func (p Postgres) createSchemas(ac4tor *DatabaseUser) error {
	for _, s := range p.Schemas {
		createSchema := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS \"%s\";", s)
		if err := p.executeExec(p.Database, createSchema, ac4tor); err != nil {
			p.Log.Error(err, "failed to create schema", "schema", s)
			return err
		}
	}

	return nil
}

func (p Postgres) checkSchemas(user *DatabaseUser) error {
	if p.DropPublicSchema {
		query := "SELECT 1 FROM pg_cataLog.pg_namespace WHERE nspname = 'public';"
		if p.isRowExist(p.Database, query, user.Username, user.Password) {
			return fmt.Errorf("schema public still exists")
		}
	}
	for _, s := range p.Schemas {
		query := fmt.Sprintf("SELECT 1 FROM pg_cataLog.pg_namespace WHERE nspname = '%s';", s)
		if !p.isRowExist(p.Database, query, user.Username, user.Password) {
			return fmt.Errorf("couldn't find schema %s in database %s", s, p.Database)
		}
	}
	return nil
}

func (p Postgres) addExtensions(admin *DatabaseUser) error {
	for _, ext := range p.Extensions {
		query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS \"%s\";", ext)
		err := p.executeExec(p.Database, query, admin)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p Postgres) enableMonitoring(admin *DatabaseUser) error {
	monitoringExtension := "pg_stat_statements"

	query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS \"%s\";", monitoringExtension)
	err := p.executeExec(p.Database, query, admin)
	if err != nil {
		return err
	}

	return nil
}

func (p Postgres) checkExtensions(user *DatabaseUser) error {
	for _, ext := range p.Extensions {
		query := fmt.Sprintf("SELECT 1 FROM pg_extension WHERE extname = '%s';", ext)
		if !p.isRowExist(p.Database, query, user.Username, user.Password) {
			return fmt.Errorf("couldn't find extension %s in database %s", ext, p.Database)
		}
	}

	return nil
}

// Functions that implement the `Database` interface

// CheckStatus checks status of postgres database
// if the connection to database works
func (p Postgres) CheckStatus(user *DatabaseUser) error {
	db, err := p.getDbConn(p.Database, user.Username, user.Password)
	if err != nil {
		return fmt.Errorf("db conn test failed - couldn't get db conn: %s", err)
	}
	defer db.Close()
	res, err := db.Query("SELECT 1")
	if err != nil {
		return fmt.Errorf("db conn test failed - failed to execute query: %s", err)
	}
	res.Close()

	if err := p.checkSchemas(user); err != nil {
		return err
	}

	if err := p.checkExtensions(user); err != nil {
		return err
	}

	return nil
}

// GetCredentials returns credentials of the postgres database
func (p Postgres) GetCredentials(user *DatabaseUser) Credentials {
	return Credentials{
		Name:     p.Database,
		Username: user.Username,
		Password: user.Password,
	}
}

// ParseAdminCredentials parse admin username and password of postgres database from secret data
// If "user" key is not defined, take "postgres" as admin user by default
func (p Postgres) ParseAdminCredentials(data map[string][]byte) (*DatabaseUser, error) {
	admin := &DatabaseUser{}

	_, ok := data["user"]
	if ok {
		admin.Username = string(data["user"])
	} else {
		// default admin username is "postgres"
		admin.Username = "postgres"
	}

	// if "password" key is defined in data, take value as password
	_, ok = data["password"]
	if ok {
		admin.Password = string(data["password"])
		return admin, nil
	}

	// take value of "postgresql-password" key as password if "password" key is not defined in data
	// it's compatible with secret created by stable postgres chart
	_, ok = data["postgresql-password"]
	if ok {
		admin.Password = string(data["postgresql-password"])
		return admin, nil
	}

	// take value of "postgresql-password" key as password if "postgresql-password" and "password" key is not defined in data
	// it's compatible with secret created by stable postgres chart
	_, ok = data["postgresql-postgres-password"]
	if ok {
		admin.Password = string(data["postgresql-postgres-password"])
		return admin, nil
	}

	return admin, errors.New("can not find postgres admin credentials")
}

func (p Postgres) GetDatabaseAddress() DatabaseAddress {
	return DatabaseAddress{
		Host: p.Host,
		Port: p.Port,
	}
}

func (p Postgres) QueryAsUser(query string, user *DatabaseUser) (string, error) {
	db, err := p.getDbConn(p.Database, user.Username, user.Password)
	if err != nil {
		p.Log.Error(err, "failed to open a db connection")
		return "", err
	}
	defer db.Close()

	var result string
	if err := db.QueryRow(query).Scan(&result); err != nil {
		p.Log.Error(err, "failed executing query", "query", query)
		return "", err
	}
	return result, nil
}

func (p Postgres) createDatabase(admin *DatabaseUser) error {
	var create string
	if len(p.Template) > 0 {
		p.Log.Info("Creating database from template", "database", p.Database, "template", p.Template)
		create = fmt.Sprintf("CREATE DATABASE \"%s\" TEMPLATE \"%s\";", p.Database, p.Template)
	} else {
		create = fmt.Sprintf("CREATE DATABASE \"%s\";", p.Database)
	}

	if !p.isDbExist(admin) {
		err := p.executeExec("postgres", create, admin)
		if err != nil {
			p.Log.Error(err, "failed creating postgres database", err)
			return err
		}
	}

	if p.Monitoring {
		err := p.enableMonitoring(admin)
		if err != nil {
			return fmt.Errorf("can not enable monitoring - %s", err)
		}
	}

	err := p.addExtensions(admin)
	if err != nil {
		return fmt.Errorf("can not add extension - %s", err)
	}

	if p.DropPublicSchema {
		if err := p.dropPublicSchema(admin); err != nil {
			return fmt.Errorf("can not drop public schema - %s", err)
		}
		if len(p.Schemas) == 0 {
			p.Log.Info("the public schema is dropped, but no additional schemas are created, schema creation must be handled on the application side now")
		}
	}

	if len(p.Schemas) > 0 {
		if err := p.createSchemas(admin); err != nil {
			p.Log.Error(err, "failed creating additional schemas")
			return err
		}
	}

	return nil
}

func (p Postgres) deleteDatabase(admin *DatabaseUser) error {
	revoke := fmt.Sprintf("REVOKE CONNECT ON DATABASE \"%s\" FROM PUBLIC, \"%s\";", p.Database, admin.Username)
	delete := fmt.Sprintf("DROP DATABASE \"%s\";", p.Database)

	if p.isDbExist(admin) {
		err := p.executeExec("postgres", revoke, admin)
		if err != nil {
			p.Log.Error(err, "failed revoking connection on database", "connection", revoke)
			return err
		}

		err = kci.Retry(3, 5*time.Second, func() error {
			err := p.executeExec("postgres", delete, admin)
			if err != nil {
				// This error will result in a retry
				p.Log.V(2).Info("failed with error, retrying", "error", err)
				return err
			}

			return nil
		})
		if err != nil {
			p.Log.V(2).Info("failed with error, retrying", "error", err)
			return err
		}
	}
	return nil
}

func (p Postgres) createOrUpdateUser(admin *DatabaseUser, user *DatabaseUser) error {
	if !p.isUserExist(admin, user) {
		if err := p.createUser(admin, user); err != nil {
			p.Log.Error(err, "failed creating postgres user")
			return err
		}
	} else {
		if err := p.updateUser(admin, user); err != nil {
			p.Log.Error(err, "failed updating postgres user")
			return err
		}
	}

	if err := p.setUserPermission(admin, user); err != nil {
		return err
	}
	return nil
}

func (p Postgres) createUser(admin *DatabaseUser, user *DatabaseUser) error {
	create := fmt.Sprintf("CREATE USER \"%s\" WITH ENCRYPTED PASSWORD '%s' NOSUPERUSER;", user.Username, user.Password)

	if !p.isUserExist(admin, user) {
		err := p.executeExec("postgres", create, admin)
		if err != nil {
			p.Log.Error(err, "failed creating postgres user")
			return err
		}
	} else {
		err := fmt.Errorf("user already exists: %s", user.Username)
		return err
	}

	if err := p.setUserPermission(admin, user); err != nil {
		return err
	}

	return nil
}

func (p Postgres) updateUser(admin *DatabaseUser, user *DatabaseUser) error {
	update := fmt.Sprintf("ALTER ROLE \"%s\" WITH ENCRYPTED PASSWORD '%s';", user.Username, user.Password)

	if !p.isUserExist(admin, user) {
		err := fmt.Errorf("user doesn't exist yet: %s", user.Username)
		return err
	} else {
		err := p.executeExec("postgres", update, admin)
		if err != nil {
			p.Log.Error(err, "failed updating postgres user", "query", update)
			return err
		}
	}

	if err := p.setUserPermission(admin, user); err != nil {
		return err
	}
	return nil
}

func (p Postgres) setUserPermission(admin *DatabaseUser, user *DatabaseUser) error {
	schemas := p.Schemas
	if !p.DropPublicSchema {
		schemas = append(schemas, "public")
	}

	// Grant user role to the admin user. It's required to make generic instances work with Azure.
	assignRoleToAdmin := fmt.Sprintf("GRANT \"%s\" TO \"%s\";", user.Username, admin.Username)
	if err := p.executeExec(p.Database, assignRoleToAdmin, admin); err != nil {
		p.Log.Error(err, "failed granting user to admin", "username", user.Username, "admin", admin.Username)
	}

	switch user.AccessType {
	case ACCESS_TYPE_MAINUSER:
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\";", p.Database, user.Username)
		err := p.executeExec("postgres", grant, admin)
		if err != nil {
			p.Log.Error(err, "failed granting all priveleges to user", "query", grant)
			return err
		}
		grantCreateToAdmin := fmt.Sprintf("GRANT CREATE ON DATABASE \"%s\" to \"%s\";", p.Database, admin.Username)
		if err := p.executeExec(p.Database, grantCreateToAdmin, admin); err != nil {
			p.Log.Error(err, "failed to grant usage access on database", "username", user.Username, "database", p.Database)
			return err
		}

		for _, s := range schemas {
			grantUserAccess := fmt.Sprintf("GRANT ALL ON SCHEMA \"%s\" TO \"%s\"", s, user.Username)
			if err := p.executeExec(p.Database, grantUserAccess, admin); err != nil {
				p.Log.Error(err, "failed to grant usage access on schema", "username", user.Username, "schema", s)
				return err
			}
		}
	case ACCESS_TYPE_READWRITE:
		for _, s := range schemas {
			grantUsage := fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"", s, user.Username)
			grantTables := fmt.Sprintf("GRANT SELECT, INSERT, DELETE, UPDATE ON ALL TABLES IN SCHEMA \"%s\" TO \"%s\"", s, user.Username)
			defaultPrivileges := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE \"%s\" IN SCHEMA \"%s\" GRANT SELECT, INSERT, DELETE, UPDATE ON TABLES TO \"%s\";",
				p.MainUser.Username,
				s,
				user.Username,
			)
			err := p.executeExec(p.Database, grantUsage, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", grantTables)
				return err
			}
			err = p.executeExec(p.Database, grantTables, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", grantTables)
				return err
			}
			err = p.executeExec(p.Database, defaultPrivileges, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", defaultPrivileges)
				return err
			}
		}
	case ACCESS_TYPE_READONLY:
		for _, s := range schemas {
			grantUsage := fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"", s, user.Username)
			grantTables := fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA \"%s\" TO \"%s\"", s, user.Username)
			defaultPrivileges := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE \"%s\" IN SCHEMA \"%s\" GRANT SELECT ON TABLES TO \"%s\";",
				p.MainUser.Username,
				s,
				user.Username,
			)
			err := p.executeExec(p.Database, grantUsage, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", grantTables)
				return err
			}
			err = p.executeExec(p.Database, grantTables, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", grantTables)
				return err
			}
			err = p.executeExec(p.Database, defaultPrivileges, admin)
			if err != nil {
				p.Log.Error(err, "failed updating postgres user", "query", defaultPrivileges)
				return err
			}
		}
	default:
		err := fmt.Errorf("unknown access type: %s", user.AccessType)
		return err
	}

	return nil
}

func (p Postgres) deleteUser(admin *DatabaseUser, user *DatabaseUser) error {
	if user.AccessType != ACCESS_TYPE_MAINUSER && p.isUserExist(admin, user) {
		schemas := p.Schemas
		if !p.DropPublicSchema {
			schemas = append(schemas, "public")
		}
		for _, schema := range schemas {
			revokeDefaults := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE \"%s\" IN SCHEMA \"%s\" REVOKE ALL ON TABLES FROM \"%s\";",
				p.MainUser.Username,
				schema,
				user.Username,
			)
			if err := p.executeExec(p.Database, revokeDefaults, p.MainUser); err != nil {
				p.Log.Error(err, "failed removing default privileges from schema", "username", user.Username, "schema", schema)
				return err
			}
			revokeAll := fmt.Sprintf("REVOKE ALL ON SCHEMA \"%s\" FROM \"%s\";", schema, user.Username)
			if err := p.executeExec(p.Database, revokeAll, admin); err != nil {
				p.Log.Error(err, "failed revoking privileges from schema", "username", user.Username, "schema", schema)
				return err
			}
			dropOwned := fmt.Sprintf("DROP OWNED BY \"%s\";", user.Username)
			if err := p.executeExec(p.Database, dropOwned, admin); err != nil {
				p.Log.Error(err, "failed dropping owned", "username", user.Username, err)
				return err
			}

		}
	}
	delete := fmt.Sprintf("DROP USER \"%s\";", user.Username)
	if p.isUserExist(admin, user) {
		err := p.executeExec("postgres", delete, admin)
		if err != nil {
			pqErr := err.(*pq.Error)
			if pqErr.Code == "2BP01" {
				// 2BP01 dependent_objects_still_exist
				p.Log.Error(err, "dependent objects still exist")
				return nil
			}
			return err
		}
	}
	return nil
}
