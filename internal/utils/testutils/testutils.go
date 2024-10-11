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

package testutils

import (
	kindav1beta2 "github.com/db-operator/db-operator/api/v1beta2"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/db-operator/db-operator/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestSecretName = "TestSec"
	TestNamespace  = "TestNS"
)

func NewPostgresTestDbInstanceCr() kindav1beta2.DbInstance {
	return kindav1beta2.DbInstance{
		Spec: kindav1beta2.DbInstanceSpec{
			Engine: "postgres",
			InstanceData: &kindav1beta2.InstanceData{
				Host: test.GetPostgresHost(),
				Port: test.GetPostgresPort(),
			},
		},
		Status: kindav1beta2.DbInstanceStatus{
			URL:  "postgres",
			Port: 5432,
		},
	}
}

func NewMysqlTestDbInstanceCr() kindav1beta2.DbInstance {
	info := make(map[string]string)
	info["DB_PORT"] = "3306"
	info["DB_CONN"] = "mysql"
	return kindav1beta2.DbInstance{
		Spec: kindav1beta2.DbInstanceSpec{
			Engine: "mysql",
			InstanceData: &kindav1beta2.InstanceData{
				Host: "mysql",
				Port: 3306,
			},
		},
		Status: kindav1beta2.DbInstanceStatus{
			URL:  "mysql",
			Port: 3306,
		},
	}
}

func NewPostgresTestDbCr(instanceRef kindav1beta2.DbInstance) *kindav1beta2.Database {
	o := metav1.ObjectMeta{Namespace: TestNamespace}
	s := kindav1beta2.DatabaseSpec{
		Credentials: kindav1beta2.Credentials{
			SecretName: TestSecretName,
		},
	}

	db := kindav1beta2.Database{
		ObjectMeta: o,
		Spec:       s,
		Status: kindav1beta2.DatabaseStatus{
			Engine: consts.ENGINE_POSTGRES,
		},
	}

	return &db
}

func NewMysqlTestDbCr() *kindav1beta2.Database {
	o := metav1.ObjectMeta{Namespace: "TestNS"}
	s := kindav1beta2.DatabaseSpec{
		Credentials: kindav1beta2.Credentials{
			SecretName: "TestSec",
		},
	}

	info := make(map[string]string)
	info["DB_PORT"] = "3306"
	info["DB_CONN"] = "mysql"

	db := kindav1beta2.Database{
		ObjectMeta: o,
		Spec:       s,
		Status: kindav1beta2.DatabaseStatus{
			Engine: consts.ENGINE_MYSQL,
		},
	}

	return &db
}
