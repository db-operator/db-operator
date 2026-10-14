/*
 * Copyright 2026 DB-Operator Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/pkg/test"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// This regression uses only Reconcile and existing API fields so the same test
// can run against the controller before and after the checkpoint fix.
func TestIntegrationDbUserDeleteRetryAfterParentRemoval(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	databaseName := "dbuser_retry_" + unique
	userName := "dbuser_retry_user_" + unique
	dsn := &url.URL{
		Scheme: "postgres", Host: net.JoinHostPort(test.GetPostgresHost(), strconv.Itoa(int(test.GetPostgresPort()))),
		User: url.UserPassword(test.GetPostgresAdminUsername(), test.GetPostgresAdminPassword()),
		Path: "postgres", RawQuery: "sslmode=disable&connect_timeout=5",
	}
	postgres, err := sql.Open("postgres", dsn.String())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, postgres.Close()) })
	require.NoError(t, postgres.PingContext(ctx), "start an isolated PostgreSQL server before running this test")
	t.Cleanup(func() {
		_, err := postgres.ExecContext(ctx, "DROP DATABASE IF EXISTS "+pq.QuoteIdentifier(databaseName))
		assert.NoError(t, err)
		_, err = postgres.ExecContext(ctx, "DROP ROLE IF EXISTS "+pq.QuoteIdentifier(userName))
		assert.NoError(t, err)
	})
	_, err = postgres.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(databaseName))
	require.NoError(t, err)
	_, err = postgres.ExecContext(ctx, "CREATE ROLE "+pq.QuoteIdentifier(userName)+" LOGIN")
	require.NoError(t, err)

	useExistingCluster := false
	environment := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &useExistingCluster,
	}
	configuration, err := environment.Start()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, environment.Stop()) })
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kindav1beta1.AddToScheme(scheme))
	cli, err := client.New(configuration, client.Options{Scheme: scheme})
	require.NoError(t, err)
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dbuser-retry-" + unique}}
	require.NoError(t, cli.Create(ctx, namespace))
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin", Namespace: namespace.Name},
		Data: map[string][]byte{
			"user": []byte(test.GetPostgresAdminUsername()), "password": []byte(test.GetPostgresAdminPassword()),
		},
	}
	require.NoError(t, cli.Create(ctx, adminSecret))
	instance := &kindav1beta1.DbInstance{
		ObjectMeta: metav1.ObjectMeta{Name: namespace.Name},
		Spec: kindav1beta1.DbInstanceSpec{
			Engine: "postgres", AdminUserSecret: kindav1beta1.NamespacedName{Name: adminSecret.Name, Namespace: namespace.Name},
			DbInstanceSource: kindav1beta1.DbInstanceSource{Generic: &kindav1beta1.GenericInstance{
				Host: test.GetPostgresHost(), Port: test.GetPostgresPort(),
			}},
		},
	}
	require.NoError(t, cli.Create(ctx, instance))
	instance.Status.Info = map[string]string{"DB_CONN": test.GetPostgresHost(), "DB_PORT": strconv.Itoa(int(test.GetPostgresPort()))}
	require.NoError(t, cli.Status().Update(ctx, instance))
	finalizer := "dbuser.reader"
	db := &kindav1beta1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: namespace.Name, Finalizers: []string{finalizer}},
		Spec:       kindav1beta1.DatabaseSpec{Instance: instance.Name, SecretName: "database-credentials"},
	}
	require.NoError(t, cli.Create(ctx, db))
	db.Status.Engine = "postgres"
	db.Status.DatabaseName = databaseName
	require.NoError(t, cli.Status().Update(ctx, db))
	user := &kindav1beta1.DbUser{
		ObjectMeta: metav1.ObjectMeta{Name: "reader", Namespace: namespace.Name, Finalizers: []string{finalizer}},
		Spec:       kindav1beta1.DbUserSpec{DatabaseRef: db.Name, SecretName: "missing-credentials", AccessType: "readWrite"},
	}
	require.NoError(t, cli.Create(ctx, user))
	user.Status.UserName = userName
	user.Status.Created = true
	require.NoError(t, cli.Status().Update(ctx, user))
	// Only the user's dependency finalizer remains on the parent. The API server
	// will remove the parent as soon as the user controller releases it.
	require.NoError(t, cli.Delete(ctx, db))
	require.NoError(t, cli.Delete(ctx, user))

	faults := &dbUserFinalizerConflictClient{Client: cli, finalizer: finalizer}
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)}
	for attempt := 0; attempt < 4 && !faults.fired; attempt++ {
		reconciler := &DbUserReconciler{Client: faults, Scheme: scheme, Recorder: events.NewFakeRecorder(100)}
		_, err = reconciler.Reconcile(ctx, request)
		if faults.fired {
			require.True(t, k8serrors.IsConflict(err), "expected a real resourceVersion conflict, got %v", err)
		} else {
			require.NoError(t, err)
		}
	}
	require.True(t, faults.fired, "the controller did not attempt to release the child finalizer")
	require.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(db), &kindav1beta1.Database{})))
	var roleExists bool
	require.NoError(t, postgres.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", userName).Scan(&roleExists))
	require.False(t, roleExists, "the external role must be gone before the parent is released")
	remaining := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, request.NamespacedName, remaining))
	require.Contains(t, remaining.Finalizers, finalizer)
	require.Equal(t, "preserved", remaining.Annotations["example.com/concurrent-update"])
	t.Log("PostgreSQL role and Database are gone; a resourceVersion conflict retained the DbUser finalizer")

	require.NoError(t, cli.Delete(ctx, instance))
	require.NoError(t, cli.Delete(ctx, adminSecret))
	restarted := &DbUserReconciler{Client: cli, Scheme: scheme, Recorder: events.NewFakeRecorder(100)}
	_, err = restarted.Reconcile(ctx, request)
	require.NoError(t, err, "a restarted controller must finish deletion after the parent disappears")
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, request.NamespacedName, &kindav1beta1.DbUser{})))
}

type dbUserFinalizerConflictClient struct {
	client.Client
	finalizer string
	fired     bool
}

func (c *dbUserFinalizerConflictClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if user, ok := obj.(*kindav1beta1.DbUser); ok && !c.fired && !slices.Contains(user.Finalizers, c.finalizer) {
		current := &kindav1beta1.DbUser{}
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(user), current); err != nil {
			return err
		}
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations["example.com/concurrent-update"] = "preserved"
		if err := c.Client.Update(ctx, current); err != nil {
			return fmt.Errorf("inject concurrent update: %w", err)
		}
		c.fired = true
	}
	return c.Client.Update(ctx, obj, opts...)
}
