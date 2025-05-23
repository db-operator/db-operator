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
	"errors"
	"testing"

	"bou.ke/monkey"
	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/internal/helpers/common"
	"github.com/db-operator/db-operator/v2/internal/utils/testutils"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/db-operator/db-operator/v2/pkg/test"
	"github.com/db-operator/db-operator/v2/pkg/utils/kci"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func testConfigmap1(_ context.Context, nsName types.NamespacedName) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	cm.Namespace = nsName.Namespace
	cm.Name = nsName.Name

	data := make(map[string]string)
	data["config"] = "test1"
	cm.Data = data

	return cm, nil
}

func testConfigmap2(_ context.Context, nsName types.NamespacedName) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	cm.Namespace = nsName.Namespace
	cm.Name = nsName.Name

	data := make(map[string]string)
	data["config"] = "test2"
	cm.Data = data

	return cm, nil
}

func errorConfigmap(namespace, configmapName string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	return cm, errors.New("whatever error")
}

func testAdminSecret(namespace, secretName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	data := make(map[string][]byte)
	data["user"] = []byte("user")
	data["password"] = []byte("securepassword")

	secret.Data = data
	return secret, nil
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
	common.AddDBInstanceChecksumStatus(ctx, dbin)
	nochange := common.IsDBInstanceSpecChanged(ctx, dbin)
	assert.Equal(t, nochange, false, "expected false")

	after := kindav1beta1.DbInstanceSpec{
		AdminUserSecret: kindav1beta1.NamespacedName{
			Namespace: "test",
			Name:      "secret2",
		},
	}
	dbin.Spec = after
	change := common.IsDBInstanceSpecChanged(ctx, dbin)
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

	patch := monkey.Patch(kci.GetConfigResource, testConfigmap1)
	defer patch.Unpatch()
	common.AddDBInstanceChecksumStatus(context.Background(), dbin)

	ctx := context.Background()

	nochange := common.IsDBInstanceSpecChanged(ctx, dbin)
	assert.Equal(t, nochange, false, "expected false")

	patch = monkey.Patch(kci.GetConfigResource, testConfigmap2)
	change := common.IsDBInstanceSpecChanged(ctx, dbin)
	assert.Equal(t, change, true, "expected true")
}

func TestAddChecksumStatus(t *testing.T) {
	dbin := &kindav1beta1.DbInstance{}
	common.AddDBInstanceChecksumStatus(context.Background(), dbin)
	checksums := dbin.Status.Checksums
	assert.NotEqual(t, checksums, map[string]string{}, "annotation should have checksum")
}
