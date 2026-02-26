//go:build !tests
// +build !tests

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

package common_test

import (
	"context"
	"testing"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/helpers/common"
	"github.com/db-operator/db-operator/v2/internal/utils/testutils"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestSecretName = "TestSec"
	TestNamespace  = "TestNS"
)

func newPostgresTestDbInstanceCr() kindav1beta1.DbInstance {
	info := make(map[string]string)
	info["DB_PORT"] = "5432"
	info["DB_CONN"] = "postgres"
	return kindav1beta1.DbInstance{
		Spec: kindav1beta1.DbInstanceSpec{
			Engine: "postgres",
			DbInstanceSource: kindav1beta1.DbInstanceSource{
				Generic: &kindav1beta1.GenericInstance{
					Host: test.GetPostgresHost(),
					Port: test.GetPostgresPort(),
				},
			},
		},
		Status: kindav1beta1.DbInstanceStatus{Info: info},
	}
}

func newMysqlTestDbInstanceCr() kindav1beta1.DbInstance {
	info := make(map[string]string)
	info["DB_PORT"] = "3306"
	info["DB_CONN"] = "mysql"
	return kindav1beta1.DbInstance{
		Spec: kindav1beta1.DbInstanceSpec{
			Engine: "mysql",
			DbInstanceSource: kindav1beta1.DbInstanceSource{
				Generic: &kindav1beta1.GenericInstance{
					Host: test.GetMysqlHost(),
					Port: test.GetMysqlPort(),
				},
			},
		},
		Status: kindav1beta1.DbInstanceStatus{Info: info},
	}
}

func newPostgresTestDbCr() *kindav1beta1.Database {
	o := metav1.ObjectMeta{Namespace: TestNamespace}
	s := kindav1beta1.DatabaseSpec{SecretName: TestSecretName}

	db := kindav1beta1.Database{
		ObjectMeta: o,
		Spec:       s,
		Status:     kindav1beta1.DatabaseStatus{Engine: consts.ENGINE_POSTGRES},
	}

	return &db
}

func newMysqlTestDbCr() *kindav1beta1.Database {
	o := metav1.ObjectMeta{Namespace: "TestNS"}
	s := kindav1beta1.DatabaseSpec{SecretName: "TestSec"}

	info := make(map[string]string)
	info["DB_PORT"] = "3306"
	info["DB_CONN"] = "mysql"

	db := kindav1beta1.Database{
		ObjectMeta: o,
		Spec:       s,
		Status:     kindav1beta1.DatabaseStatus{Engine: consts.ENGINE_MYSQL},
	}

	return &db
}

func TestIsSpecChanged(t *testing.T) {
	db := testutils.NewPostgresTestDbCr(testutils.NewPostgresTestDbInstanceCr())

	testDbSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: testutils.TestNamespace, Name: testutils.TestSecretName},
		Data: map[string][]byte{
			"POSTGRES_DB":       []byte("testdb"),
			"POSTGRES_USER":     []byte("testuser"),
			"POSTGRES_PASSWORD": []byte("testpassword"),
		},
	}

	common.AddDBChecksum(db, testDbSecret)
	nochange := common.IsDBChanged(db, testDbSecret)
	assert.Equal(t, nochange, false, "expected false")

	testDbSecret.Data["POSTGRES_PASSWORD"] = []byte("testpasswordNEW")

	change := common.IsDBChanged(db, testDbSecret)
	assert.Equal(t, change, true, "expected true")
}

func TestSpecChanged(t *testing.T) {
	dbin := &kindav1beta1.DbInstance{}
	before := kindav1beta1.DbInstanceSpec{
		AdminUserSecret: kindav1beta1.NamespacedName{
			Namespace: "test",
			Name:      "secret1",
		},
	}

	ctx := context.Background()

	dbin.Spec = before
	// we pretend this data comes from the secret referenced above
	data := common.DbInstanceData{
		AdminSecret: &corev1.Secret{
			Data: map[string][]byte{"user": []byte("user1")},
		},
	}
	common.AddDBInstanceChecksumStatus(ctx, dbin, data)
	nochange := common.IsDBInstanceSpecChanged(ctx, dbin, data)
	assert.Equal(t, nochange, false, "expected false")

	after := kindav1beta1.DbInstanceSpec{
		AdminUserSecret: kindav1beta1.NamespacedName{
			Namespace: "test",
			Name:      "secret2",
		},
	}
	dbin.Spec = after
	// referene changed, but secret data is the same
	change := common.IsDBInstanceSpecChanged(ctx, dbin, data)
	assert.Equal(t, change, true, "expected true")
}

func TestConfigChanged(t *testing.T) {
	dbin := &kindav1beta1.DbInstance{}
	dbin.Spec.Google = &kindav1beta1.GoogleInstance{
		InstanceName: "test",
		ConfigmapName: kindav1beta1.NamespacedName{
			Namespace: "testNS",
			Name:      "test",
		},
	}

	ctx := context.Background()
	// pretend this is the data from "testNS/test" configmap
	data1 := common.DbInstanceData{
		ConfigMap: &corev1.ConfigMap{
			Data: map[string]string{"config": "test1"},
		},
	}

	common.AddDBInstanceChecksumStatus(ctx, dbin, data1)

	nochange := common.IsDBInstanceSpecChanged(ctx, dbin, data1)
	assert.Equal(t, nochange, false, "expected false")

	data2 := common.DbInstanceData{
		AdminSecret: &corev1.Secret{
			Data: map[string][]byte{"user": []byte("user1")},
		},
		ConfigMap: &corev1.ConfigMap{
			Data: map[string]string{"config": "test2"},
		},
	}
	change := common.IsDBInstanceSpecChanged(ctx, dbin, data2)
	assert.Equal(t, change, true, "expected true")
}

func TestAddChecksumStatus(t *testing.T) {
	dbin := &kindav1beta1.DbInstance{}
	data := common.DbInstanceData{
		AdminSecret: &corev1.Secret{
			Data: map[string][]byte{"user": []byte("user1")},
		},
	}
	common.AddDBInstanceChecksumStatus(context.Background(), dbin, data)
	checksums := dbin.Status.Checksums
	assert.NotEqual(t, checksums, map[string]string{}, "annotation should have checksum")
}
