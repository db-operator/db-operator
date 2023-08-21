/*
 * Copyright 2021 kloeckner.i GmbH
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

// CreateDatabase executes queries to create database
func CreateDatabase(db Database, admin *DatabaseUser) error {
	err := db.createDatabase(admin)
	if err != nil {
		return err
	}
	return nil
}

// DeleteDatabase executes queries to delete database and user
func DeleteDatabase(db Database, admin *DatabaseUser) error {
	err := db.deleteDatabase(admin)
	if err != nil {
		return err
	}

	return nil
}

// CreateOrUpdateUser executes queries to create or update user
func CreateOrUpdateUser(db Database, dbuser *DatabaseUser, admin *DatabaseUser) error {
	err := db.createOrUpdateUser(admin, dbuser)
	if err != nil {
		return err
	}
	return nil
}

// CreateUser executes queries to a create user
func CreateUser(db Database, dbuser *DatabaseUser, admin *DatabaseUser) error {
	err := db.createUser(admin, dbuser)
	if err != nil {
		return err
	}
	return nil
}

func UpdateUser(db Database, dbuser *DatabaseUser, admin *DatabaseUser) error {
	err := db.createOrUpdateUser(admin, dbuser)
	if err != nil {
		return err
	}
	return nil
}

func DeleteUser(db Database, dbuser *DatabaseUser, admin *DatabaseUser) error {
	err := db.deleteUser(admin, dbuser)
	if err != nil {
		return err
	}

	return nil
}

// New returns database interface according to engine type
func New(engine string) Database {
	switch engine {
	case "postgres":
		return &Postgres{}
	case "mysql":
		return &Mysql{}
	case "dummy":
		return &Dummy{}
	}

	return nil
}
