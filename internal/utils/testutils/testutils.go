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
	"strconv"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestSecretName = "TestSec"
	TestNamespace  = "TestNS"
)

var (
	postgresEngine= "postgres"
	mysqlEngine = "mysql"
)

func NewPostgresTestDbInstanceCr() kindav1.DbInstance {
	host := test.GetPostgresHost()
	port := test.GetPostgresPort()
	portStr := strconv.FormatUint(uint64(port), 10)

	return kindav1.DbInstance{
		Spec: kindav1.DbInstanceSpec{
			Engine: &postgresEngine,
			Endpoint: &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{
					Value: &host,
				},
				Port: &kindav1.ValueSource{
					Value: &portStr,
				},
				SSLConnection: &kindav1.DbInstanceSSLConnection{},
			},
		},
		Status: kindav1.DbInstanceStatus{
			MainEndpoint: &kindav1.DbInstanceServerData{
				Host: host,
				Port: port,
				SSLConnection: kindav1.DbInstanceSSLConnection{},
			},
		},
	}
}

func NewMysqlTestDbInstanceCr() kindav1.DbInstance {
	host := test.GetMysqlHost()
	port := test.GetMysqlPort()
	portStr := strconv.FormatUint(uint64(port), 10)

	return kindav1.DbInstance{
		Spec: kindav1.DbInstanceSpec{
			Engine: &mysqlEngine,
			Endpoint: &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{
					Value: &host,
				},
				Port: &kindav1.ValueSource{
					Value: &portStr,
				},
				SSLConnection: &kindav1.DbInstanceSSLConnection{},
			},
		},
		Status: kindav1.DbInstanceStatus{
			MainEndpoint: &kindav1.DbInstanceServerData{
				Host: host,
				Port: port,
				SSLConnection: kindav1.DbInstanceSSLConnection{},
			},
		},
	}
}

func NewPostgresTestDbCr() *kindav1beta1.Database {
	o := metav1.ObjectMeta{Namespace: TestNamespace}
	s := kindav1beta1.DatabaseSpec{SecretName: TestSecretName}

	db := kindav1beta1.Database{
		ObjectMeta: o,
		Spec:       s,
		Status: kindav1beta1.DatabaseStatus{
			Engine: consts.ENGINE_POSTGRES,
		},
	}

	return &db
}

func NewMysqlTestDbCr() *kindav1beta1.Database {
	o := metav1.ObjectMeta{Namespace: "TestNS"}
	s := kindav1beta1.DatabaseSpec{SecretName: "TestSec"}

	info := make(map[string]string)
	info["DB_PORT"] = "3306"
	info["DB_CONN"] = "mysql"

	db := kindav1beta1.Database{
		ObjectMeta: o,
		Spec:       s,
		Status: kindav1beta1.DatabaseStatus{
			Engine: consts.ENGINE_MYSQL,
		},
	}

	return &db
}
