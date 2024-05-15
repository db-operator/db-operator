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

package v1beta1_test

import (
	"testing"

	"github.com/db-operator/db-operator/api/v1beta1"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/stretchr/testify/assert"
)

func TestExtraPrivilegesFail1(t *testing.T) {
	privileges := []string{consts.ALL_PRIVILEGES}
	err := v1beta1.TestExtraPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestExtraPrivilegesFail2(t *testing.T) {
	privileges := []string{"all privileges"}
	err := v1beta1.TestExtraPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestExtraPrivilegesFail3(t *testing.T) {
	privileges := []string{"aLL PriVileges"}
	err := v1beta1.TestExtraPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestExtraPrivileges(t *testing.T) {
	privileges := []string{"rds_admin"}
	err := v1beta1.TestExtraPrivileges(privileges)
	assert.NoError(t, err)
}
