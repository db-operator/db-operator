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

package dbinstance_test

import (
	"testing"

	"github.com/db-operator/db-operator/v2/pkg/utils/database"
	"github.com/db-operator/db-operator/v2/pkg/utils/dbinstance"
	"github.com/stretchr/testify/assert"
)

func TestPostgresDetect(t *testing.T) {
	instance := dbinstance.TestGenericPostgresInstance()
	dbuser := &database.DatabaseUser{
		Username: instance.User,
		Password: instance.Password,
	}
	engine, err := dbinstance.DetectEngine(t.Context(), instance, dbuser)
	assert.NoError(t, err)
	assert.Equal(t, "postgres", engine.Engine)
}

func TestMysqlDetect(t *testing.T) {
	instance := dbinstance.TestGenericMysqlInstance()
	dbuser := &database.DatabaseUser{
		Username: instance.User,
		Password: instance.Password,
	}
	engine, err := dbinstance.DetectEngine(t.Context(), instance, dbuser)
	assert.NoError(t, err)
	assert.Equal(t, "mysql", engine.Engine)
}
