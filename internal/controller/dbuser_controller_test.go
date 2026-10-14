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
	"errors"
	"testing"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	kubehelper "github.com/db-operator/db-operator/v2/internal/helpers/kube"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUnitDbUserDeleteCredentials(t *testing.T) {
	dbcr := &kindav1beta1.Database{
		Status: kindav1beta1.DatabaseStatus{
			DatabaseName: "test-database",
			Engine:       "postgres",
		},
	}

	t.Run("uses the username recorded in status", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			Status: kindav1beta1.DbUserStatus{UserName: "status-user"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "test-database", creds.Name)
		assert.Equal(t, "status-user", creds.Username)
	})

	t.Run("uses the Secret for a legacy DbUser", func(t *testing.T) {
		dbusercr, _, secret := dbUserDeletionObjects()
		dbusercr.Status.UserName = ""
		secret.Data = map[string][]byte{
			"POSTGRES_DB":       []byte("secret-database"),
			"POSTGRES_USER":     []byte("secret-user"),
			"POSTGRES_PASSWORD": []byte("password"),
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, secret)
		require.NoError(t, err)
		assert.Equal(t, "test-database", creds.Name)
		assert.Equal(t, "secret-user", creds.Username)
	})

	t.Run("derives the username when the legacy Secret is missing", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "reader"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "test-reader", creds.Username)
	})

	t.Run("uses an existing username when the legacy Secret is missing", func(t *testing.T) {
		dbusercr := &kindav1beta1.DbUser{
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "reader"},
			Spec:       kindav1beta1.DbUserSpec{ExistingUser: "existing-reader"},
		}

		creds, err := dbUserDeleteCredentials(dbusercr, dbcr, nil)
		require.NoError(t, err)
		assert.Equal(t, "existing-reader", creds.Username)
	})
}

func TestUnitDbUserDeleteCredentialsIgnoreUnownedSecret(t *testing.T) {
	for _, ownership := range []string{"released", "another user", "another UID"} {
		t.Run(ownership, func(t *testing.T) {
			user, db, secret := dbUserDeletionObjects()
			secret.Data = map[string][]byte{
				"POSTGRES_DB": []byte("another-database"), "POSTGRES_USER": []byte("another-role"), "POSTGRES_PASSWORD": []byte("password"),
			}
			switch ownership {
			case "released":
				secret.Labels = nil
			case "another user":
				secret.Labels[consts.USED_BY_NAME_LABEL_KEY] = "another-user"
			case "another UID":
				secret.OwnerReferences = []metav1.OwnerReference{{Kind: "DbUser", UID: "another-uid"}}
			}
			creds, err := dbUserDeleteCredentials(user, db, secret)
			require.NoError(t, err)
			assert.Equal(t, db.Status.DatabaseName, creds.Name)
			assert.Equal(t, user.Status.UserName, creds.Username)

			user.Status.UserName = ""
			creds, err = dbUserDeleteCredentials(user, db, secret)
			require.ErrorContains(t, err, "deletion identity is missing from status")
			assert.Empty(t, creds.Username, "an unowned Secret must not select a backend role for deletion")
		})
	}
}

func TestUnitDbUserDeleteRetries(t *testing.T) {
	for _, stage := range []string{"secret", "checkpoint", "database", "dbuser"} {
		t.Run(stage+" conflict", func(t *testing.T) {
			ctx := context.Background()
			user, db, secret := dbUserDeletionObjects()
			cli := dbUserDeletionClient(t, user, db, secret)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), db))
			faults := &dbUserDeletionFaultClient{Client: cli, stage: stage}
			r := dbUserDeletionReconciler(faults, user)

			// Backend cleanup calls have succeeded. Fail each subsequent write
			// in turn, then discard the reconciler to simulate an operator restart.
			_, err := r.completeDbUserCleanup(ctx, user, db)
			if stage != "secret" && stage != "checkpoint" {
				require.NoError(t, err)
				_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
			}
			require.True(t, k8serrors.IsConflict(err), "expected conflict, got %v", err)
			require.True(t, faults.fired)
			storedUser := &kindav1beta1.DbUser{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
			assert.Contains(t, storedUser.Finalizers, "dbuser.reader")
			storedDB := &kindav1beta1.Database{}
			dbErr := cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB)
			if stage == "dbuser" {
				require.True(t, k8serrors.IsNotFound(dbErr), "Database must disappear before the child update fails")
			} else {
				require.NoError(t, dbErr)
				assert.Contains(t, storedDB.Finalizers, "dbuser.reader")
			}

			restarted := dbUserDeletionReconciler(cli, storedUser)
			if stage == "secret" || stage == "checkpoint" {
				assert.Empty(t, storedUser.Status.CleanupDatabaseUID)
				storedSecret := &corev1.Secret{}
				require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), storedSecret))
				if stage == "secret" {
					assert.Equal(t, "reader", storedSecret.Labels[consts.USED_BY_NAME_LABEL_KEY])
				} else {
					assert.NotContains(t, storedSecret.Labels, consts.USED_BY_NAME_LABEL_KEY)
				}
				_, err = restarted.completeDbUserCleanup(ctx, storedUser, storedDB)
				require.NoError(t, err)
			} else {
				assert.Equal(t, db.UID, storedUser.Status.CleanupDatabaseUID)
				assert.Equal(t, db.Name, storedUser.Status.CleanupDatabaseName)
			}
			// No DbInstance or admin credentials exist, so a full reconciliation
			// can only succeed by resuming the persisted checkpoint.
			_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
			require.NoError(t, err)
			assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(user), &kindav1beta1.DbUser{})))
			assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(db), &kindav1beta1.Database{})))
			storedSecret := &corev1.Secret{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), storedSecret))
			assert.NotContains(t, storedSecret.Labels, consts.USED_BY_KIND_LABEL_KEY)
			assert.NotContains(t, storedSecret.Labels, consts.USED_BY_NAME_LABEL_KEY)
			assert.Equal(t, "preserved", storedSecret.Labels["example.com/label"])
			assert.Equal(t, secret.Data, storedSecret.Data)
		})
	}
}

func TestUnitDbUserDeletePrunedCheckpointKeepsFinalizers(t *testing.T) {
	for _, stage := range []string{"pruned-response", "pruned-storage"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			user, db, secret := dbUserDeletionObjects()
			cli := dbUserDeletionClient(t, user, db, secret)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
			faults := &dbUserDeletionFaultClient{Client: cli, stage: stage}
			r := dbUserDeletionReconciler(faults, user)
			_, err := r.completeDbUserCleanup(ctx, user, db)
			if stage == "pruned-response" {
				require.ErrorContains(t, err, "checkpoint was not persisted")
			} else {
				require.NoError(t, err)
			}
			require.True(t, faults.fired)
			stored := &kindav1beta1.DbUser{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), stored))
			assert.Empty(t, stored.Status.CleanupDatabaseUID)
			restarted := dbUserDeletionReconciler(cli, stored)
			_, err = restarted.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
			require.Error(t, err, "cleanup must be retried without a persisted checkpoint")
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), stored))
			assert.Equal(t, user.Finalizers, stored.Finalizers)
			storedDB := &kindav1beta1.Database{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
			assert.Equal(t, db.Finalizers, storedDB.Finalizers)
			storedSecret := &corev1.Secret{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), storedSecret))
			assert.Equal(t, map[string]string{"example.com/label": "preserved"}, storedSecret.Labels)
		})
	}
}

func TestUnitDbUserDeleteDoesNotReleaseSecretOwnedByAnotherUID(t *testing.T) {
	ctx := context.Background()
	user, db, secret := dbUserDeletionObjects()
	secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "kinda.rocks/v1beta1", Kind: "DbUser", Name: user.Name, UID: "different-user"}}
	cli := dbUserDeletionClient(t, user, db, secret)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.completeDbUserCleanup(ctx, user, db)
	require.NoError(t, err)
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	stored := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), stored))
	assert.Equal(t, secret.Labels, stored.Labels)
	assert.Equal(t, secret.OwnerReferences, stored.OwnerReferences)
}

func TestUnitDbUserDeleteMissingParent(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(map[bool]string{false: "keeps unproven finalizer", true: "resumes completed cleanup"}[completed], func(t *testing.T) {
			ctx := context.Background()
			user, db, _ := dbUserDeletionObjects()
			if completed {
				user.Status.CleanupDatabaseUID = db.UID
				user.Status.CleanupDatabaseName = db.Name
			}
			cli := dbUserDeletionClient(t, user)
			r := dbUserDeletionReconciler(cli, user)
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
			stored := &kindav1beta1.DbUser{}
			getErr := cli.Get(ctx, client.ObjectKeyFromObject(user), stored)
			if completed {
				require.NoError(t, err)
				assert.True(t, k8serrors.IsNotFound(getErr))
			} else {
				require.True(t, k8serrors.IsNotFound(err))
				require.NoError(t, getErr)
				assert.Empty(t, stored.Status.CleanupDatabaseUID)
				assert.Contains(t, stored.Finalizers, "dbuser.reader")
			}
		})
	}
}

func TestUnitDbUserDeletePreservesUnrelatedResources(t *testing.T) {
	ctx := context.Background()
	user, db, secret := dbUserDeletionObjects()
	user.Finalizers = append(user.Finalizers, "example.com/keep-user")
	db.Finalizers = append(db.Finalizers, "dbuser.other", "example.com/keep-database")
	secret.Labels[consts.USED_BY_NAME_LABEL_KEY] = "another-user"
	cli := dbUserDeletionClient(t, user, db, secret)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.completeDbUserCleanup(ctx, user, db)
	require.NoError(t, err)
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	storedUser := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
	assert.Equal(t, []string{"example.com/keep-user"}, storedUser.Finalizers)
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, []string{"dbuser.other", "example.com/keep-database"}, storedDB.Finalizers)
	storedSecret := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), storedSecret))
	assert.Equal(t, secret.Labels, storedSecret.Labels)
	assert.Equal(t, secret.Data, storedSecret.Data)
}

func TestUnitDbUserDeleteDoesNotChangeReplacementDatabase(t *testing.T) {
	ctx := context.Background()
	user, db, _ := dbUserDeletionObjects()
	user.Status.CleanupDatabaseUID = "original-database"
	user.Status.CleanupDatabaseName = db.Name
	cli := dbUserDeletionClient(t, user, db)
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, db.Finalizers, storedDB.Finalizers)
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(user), &kindav1beta1.DbUser{})))
}

func TestUnitDbUserDeleteUsesCheckpointParent(t *testing.T) {
	ctx := context.Background()
	user, db, _ := dbUserDeletionObjects()
	otherDB := db.DeepCopy()
	otherDB.Name = "other-database"
	otherDB.UID = "other-database-uid"
	cli := dbUserDeletionClient(t, user, db, otherDB)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.completeDbUserCleanup(ctx, user, db)
	require.NoError(t, err)

	user.Spec.DatabaseRef = otherDB.Name
	require.NoError(t, cli.Update(ctx, user))
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(db), &kindav1beta1.Database{})))
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(user), &kindav1beta1.DbUser{})))
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(otherDB), storedDB))
	assert.Equal(t, otherDB.Finalizers, storedDB.Finalizers)
}

func TestUnitDbUserDeleteStaleParentKeepsFinalizer(t *testing.T) {
	ctx := context.Background()
	user, db, _ := dbUserDeletionObjects()
	user.Status.CleanupDatabaseUID = db.UID
	user.Status.CleanupDatabaseName = db.Name
	db.DeletionTimestamp = nil
	db.Finalizers = []string{"example.com/keep-database"}
	cli := dbUserDeletionClient(t, user, db)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), db))
	stale := db.DeepCopy()
	db.Finalizers = append(db.Finalizers, "dbuser.reader")
	require.NoError(t, cli.Update(ctx, db))

	// The cache still shows the parent before its dependency finalizer was added.
	r := dbUserDeletionReconciler(&dbUserStaleParentClient{Client: cli, parent: stale}, user)
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.True(t, k8serrors.IsConflict(err), "expected stale parent update to conflict, got %v", err)
	storedUser := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
	assert.Contains(t, storedUser.Finalizers, "dbuser.reader")
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, db.Finalizers, storedDB.Finalizers)

	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(user), &kindav1beta1.DbUser{})))
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, []string{"example.com/keep-database"}, storedDB.Finalizers)
}

func TestUnitDbUserDeleteReleasesOriginalSecretBeforeCheckpoint(t *testing.T) {
	ctx := context.Background()
	user, db, secret := dbUserDeletionObjects()
	otherSecret := secret.DeepCopy()
	otherSecret.Name = "other-credentials"
	cli := dbUserDeletionClient(t, user, db, secret, otherSecret)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.completeDbUserCleanup(ctx, user, db)
	require.NoError(t, err)

	user.Spec.SecretName = otherSecret.Name
	require.NoError(t, cli.Update(ctx, user))
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	assert.True(t, k8serrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(user), &kindav1beta1.DbUser{})))
	stored := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), stored))
	assert.Equal(t, map[string]string{"example.com/label": "preserved"}, stored.Labels)
	assert.Equal(t, secret.Data, stored.Data)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(otherSecret), stored))
	assert.Equal(t, otherSecret.Labels, stored.Labels)
	assert.Equal(t, otherSecret.Data, stored.Data)
}

func TestUnitDbUserDeleteChecksAuthoritativeSecret(t *testing.T) {
	for _, state := range []string{"cached missing", "cached unowned", "API read failure", "adopted", "replacement"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			user, db, secret := dbUserDeletionObjects()
			secret.UID = "original-secret"
			cli := dbUserDeletionClient(t, user, db, secret)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), secret))
			cache := &dbUserSecretReadClient{Client: cli, secret: secret.DeepCopy()}
			r := dbUserDeletionReconciler(cache, user)
			r.apiReader = cli
			apiFailure := errors.New("injected API read failure")
			switch state {
			case "cached missing":
				cache.err = k8serrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, secret.Name)
			case "cached unowned", "API read failure":
				cache.secret.Labels = nil
				if state == "API read failure" {
					r.apiReader = &dbUserSecretReadClient{Client: cli, err: apiFailure}
				}
			case "adopted":
				secret.Labels[consts.USED_BY_NAME_LABEL_KEY] = "another-user"
				require.NoError(t, cli.Update(ctx, secret))
			case "replacement":
				require.NoError(t, cli.Delete(ctx, secret))
				secret.UID = "replacement-secret"
				secret.ResourceVersion = ""
				secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "kinda.rocks/v1beta1", Kind: "DbUser", Name: user.Name, UID: "another-user-uid"}}
				require.NoError(t, cli.Create(ctx, secret))
			}

			_, err := r.completeDbUserCleanup(ctx, user, db)
			storedUser := &kindav1beta1.DbUser{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
			if state == "API read failure" {
				require.ErrorIs(t, err, apiFailure)
				assert.Empty(t, storedUser.Status.CleanupDatabaseUID)
				assert.Empty(t, storedUser.Status.CleanupDatabaseName)
			} else {
				require.NoError(t, err)
				assert.Equal(t, db.UID, storedUser.Status.CleanupDatabaseUID)
				assert.Equal(t, db.Name, storedUser.Status.CleanupDatabaseName)
			}
			assert.Equal(t, user.Finalizers, storedUser.Finalizers)
			storedDB := &kindav1beta1.Database{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
			assert.Equal(t, db.Finalizers, storedDB.Finalizers)
			storedSecret := &corev1.Secret{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), storedSecret))
			if state == "cached missing" || state == "cached unowned" {
				assert.Equal(t, map[string]string{"example.com/label": "preserved"}, storedSecret.Labels)
			} else {
				assert.Equal(t, secret.Labels, storedSecret.Labels)
				assert.Equal(t, secret.ResourceVersion, storedSecret.ResourceVersion)
			}
			assert.Equal(t, secret.UID, storedSecret.UID)
			assert.Equal(t, secret.OwnerReferences, storedSecret.OwnerReferences)
			assert.Equal(t, secret.Data, storedSecret.Data)
		})
	}
}

func TestUnitDbUserDeleteTemplateRetryDoesNotReclaimSecret(t *testing.T) {
	for _, ownership := range []string{"released", "another user", "another UID"} {
		t.Run(ownership, func(t *testing.T) {
			ctx := context.Background()
			user, db, secret := dbUserDeletionObjects()
			db.Spec.SecretName = "database-credentials"
			db.Spec.Instance = "instance"
			secret.Data = map[string][]byte{
				"POSTGRES_DB": []byte(db.Status.DatabaseName), "POSTGRES_USER": []byte(user.Status.UserName), "POSTGRES_PASSWORD": []byte("password"),
			}
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: db.Spec.SecretName, Namespace: db.Namespace}}
			instance := &kindav1beta1.DbInstance{
				ObjectMeta: metav1.ObjectMeta{Name: db.Spec.Instance},
				Spec: kindav1beta1.DbInstanceSpec{DbInstanceSource: kindav1beta1.DbInstanceSource{
					Generic: &kindav1beta1.GenericInstance{Host: "localhost", Port: 5432},
				}},
				Status: kindav1beta1.DbInstanceStatus{Info: map[string]string{"DB_CONN": "localhost", "DB_PORT": "5432"}},
			}
			cli := dbUserDeletionClient(t, user, db, secret, configMap, instance)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), user))
			faults := &dbUserDeletionFaultClient{Client: cli, stage: "checkpoint"}
			r := dbUserDeletionReconciler(faults, user)
			_, err := r.completeDbUserCleanup(ctx, user, db)
			require.True(t, k8serrors.IsConflict(err))
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), secret))
			switch ownership {
			case "released":
				delete(secret.Labels, consts.USED_BY_KIND_LABEL_KEY)
				delete(secret.Labels, consts.USED_BY_NAME_LABEL_KEY)
			case "another user":
				secret.Labels[consts.USED_BY_KIND_LABEL_KEY] = "DbUser"
				secret.Labels[consts.USED_BY_NAME_LABEL_KEY] = "another-user"
			case "another UID":
				secret.Labels[consts.USED_BY_KIND_LABEL_KEY] = "DbUser"
				secret.Labels[consts.USED_BY_NAME_LABEL_KEY] = user.Name
				secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "kinda.rocks/v1beta1", Kind: "DbUser", Name: user.Name, UID: "different-user"}}
			}
			require.NoError(t, cli.Update(ctx, secret))

			// A retry must skip template writes to the released or adopted Secret.
			require.NoError(t, r.handleTemplatedCredentials(ctx, db, user, nil))
			stored := &corev1.Secret{}
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), stored))
			assert.Equal(t, secret.ResourceVersion, stored.ResourceVersion, "template retry must not write the Secret")
			assert.Equal(t, secret.Labels, stored.Labels)
			assert.Equal(t, secret.OwnerReferences, stored.OwnerReferences)
			assert.Equal(t, secret.Data, stored.Data)
			_, err = r.completeDbUserCleanup(ctx, user, db)
			require.NoError(t, err)
		})
	}
}

func TestUnitDbUserDeleteIncompleteCheckpointKeepsFinalizers(t *testing.T) {
	ctx := context.Background()
	user, db, _ := dbUserDeletionObjects()
	user.Status.CleanupDatabaseUID = db.UID
	cli := dbUserDeletionClient(t, user, db)
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.ErrorContains(t, err, "checkpoint is missing the Database name")
	storedUser := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
	assert.Equal(t, user.Finalizers, storedUser.Finalizers)
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, db.Finalizers, storedDB.Finalizers)
}

func TestUnitDbUserDeleteReleasesSecretWithoutTypeMeta(t *testing.T) {
	ctx := context.Background()
	user, _, secret := dbUserDeletionObjects()
	user.TypeMeta = metav1.TypeMeta{}
	cli := dbUserDeletionClient(t, secret)
	r := dbUserDeletionReconciler(cli, user)
	require.NoError(t, r.releaseDbUserSecret(ctx, user))
	stored := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(secret), stored))
	assert.Equal(t, map[string]string{"example.com/label": "preserved"}, stored.Labels)
}

func TestUnitDbUserDeleteWithoutOwnFinalizer(t *testing.T) {
	ctx := context.Background()
	user, _, _ := dbUserDeletionObjects()
	user.Finalizers = []string{"example.com/keep-user"}
	cli := dbUserDeletionClient(t, user)
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.NoError(t, err)
	stored := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), stored))
	assert.Equal(t, user.Finalizers, stored.Finalizers)
}

func TestUnitDbUserDeleteCleanupFailureKeepsFinalizers(t *testing.T) {
	ctx := context.Background()
	user, db, _ := dbUserDeletionObjects()
	// The missing instance prevents external cleanup from starting.
	db.Spec.Instance = "missing-instance"
	cli := dbUserDeletionClient(t, user, db)
	r := dbUserDeletionReconciler(cli, user)
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(user)})
	require.Error(t, err)
	storedUser := &kindav1beta1.DbUser{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(user), storedUser))
	assert.Empty(t, storedUser.Status.CleanupDatabaseUID)
	assert.Equal(t, user.Finalizers, storedUser.Finalizers)
	storedDB := &kindav1beta1.Database{}
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(db), storedDB))
	assert.Equal(t, db.Finalizers, storedDB.Finalizers)
}

func dbUserDeletionObjects() (*kindav1beta1.DbUser, *kindav1beta1.Database, *corev1.Secret) {
	deleted := metav1.Now()
	user := &kindav1beta1.DbUser{
		TypeMeta: metav1.TypeMeta{APIVersion: "kinda.rocks/v1beta1", Kind: "DbUser"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "reader", Namespace: "test", UID: "user-uid",
			DeletionTimestamp: &deleted, Finalizers: []string{"dbuser.reader"},
		},
		Spec:   kindav1beta1.DbUserSpec{DatabaseRef: "database", SecretName: "reader-credentials"},
		Status: kindav1beta1.DbUserStatus{UserName: "test-reader"},
	}
	db := &kindav1beta1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "database", Namespace: "test", UID: "database-uid",
			DeletionTimestamp: &deleted, Finalizers: []string{"dbuser.reader"},
		},
		Status: kindav1beta1.DatabaseStatus{DatabaseName: "test-database", Engine: "postgres"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: user.Spec.SecretName, Namespace: user.Namespace,
			Labels: map[string]string{
				consts.USED_BY_KIND_LABEL_KEY: "DbUser", consts.USED_BY_NAME_LABEL_KEY: user.Name,
				"example.com/label": "preserved",
			},
		},
		Data: map[string][]byte{"password": []byte("unchanged")},
	}
	return user, db, secret
}

func dbUserDeletionClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, kindav1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&kindav1beta1.DbUser{}, &kindav1beta1.Database{}).
		WithObjects(objects...).Build()
}

func dbUserDeletionReconciler(cli client.Client, user *kindav1beta1.DbUser) *DbUserReconciler {
	recorder := events.NewFakeRecorder(100)
	return &DbUserReconciler{
		Client: cli, Recorder: recorder,
		kubeHelper: kubehelper.NewKubeHelper(cli, recorder, user),
	}
}

type dbUserDeletionFaultClient struct {
	client.Client
	stage string
	fired bool
}

type dbUserStaleParentClient struct {
	client.Client
	parent *kindav1beta1.Database
}

func (c *dbUserStaleParentClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if db, ok := obj.(*kindav1beta1.Database); ok && c.parent != nil {
		*db = *c.parent.DeepCopy()
		c.parent = nil
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

type dbUserSecretReadClient struct {
	client.Client
	secret *corev1.Secret
	err    error
}

func (c *dbUserSecretReadClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if secret, ok := obj.(*corev1.Secret); ok {
		if c.err != nil {
			return c.err
		}
		*secret = *c.secret.DeepCopy()
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *dbUserDeletionFaultClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	stage := ""
	switch obj.(type) {
	case *corev1.Secret:
		stage = "secret"
	case *kindav1beta1.Database:
		stage = "database"
	case *kindav1beta1.DbUser:
		stage = "dbuser"
	}
	if !c.fired && c.stage == stage {
		c.fired = true
		return k8serrors.NewConflict(schema.GroupResource{Group: "kinda.rocks", Resource: stage}, obj.GetName(), errors.New("injected conflict"))
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *dbUserDeletionFaultClient) Status() client.SubResourceWriter {
	return &dbUserDeletionFaultStatus{SubResourceWriter: c.Client.Status(), client: c}
}

type dbUserDeletionFaultStatus struct {
	client.SubResourceWriter
	client *dbUserDeletionFaultClient
}

func (s *dbUserDeletionFaultStatus) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if s.client.stage == "pruned-response" || s.client.stage == "pruned-storage" {
		s.client.fired = true
		pruned := obj.(*kindav1beta1.DbUser).DeepCopy()
		pruned.Status.CleanupDatabaseUID = ""
		pruned.Status.CleanupDatabaseName = ""
		if err := s.SubResourceWriter.Update(ctx, pruned, opts...); err != nil {
			return err
		}
		if s.client.stage == "pruned-response" {
			*obj.(*kindav1beta1.DbUser) = *pruned
		} else {
			obj.SetResourceVersion(pruned.ResourceVersion)
		}
		return nil
	}
	if !s.client.fired && s.client.stage == "checkpoint" {
		s.client.fired = true
		return k8serrors.NewConflict(schema.GroupResource{Group: "kinda.rocks", Resource: "dbusers"}, obj.GetName(), errors.New("injected conflict"))
	}
	return s.SubResourceWriter.Update(ctx, obj, opts...)
}
