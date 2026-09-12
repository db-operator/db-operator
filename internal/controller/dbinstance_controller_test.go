/*
 * Copyright 2026 DB-Operator Authors
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

package controller_test

import (
	"context"
	"strconv"
	"time"

	commonhelper "github.com/db-operator/db-operator/v2/internal/helpers/common"
	"github.com/db-operator/db-operator/v2/pkg/config"
	"github.com/db-operator/db-operator/v2/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	"github.com/db-operator/db-operator/v2/internal/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("DbInstance Reconciler", func() {
	namespace := "default"

	secretName := "my-secret"
	configMapName := "my-config"

	interval := time.Duration(5 * time.Second)

	conf := &config.Config{
		DatabaseAwareness: true,
		ServerVersionTTL:  5 * time.Second,
	}
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte(test.GetPostgresAdminPassword()),
			"username": []byte(test.GetPostgresAdminUsername()),
		},
	}

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"host": test.GetPostgresHost(),
			"port": strconv.FormatUint(uint64(test.GetPostgresPort()), 10),
		},
	}

	dbinstance := &kindav1.DbInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dbinstance",
		},
		Spec: kindav1.DbInstanceSpec{
			Engine: &[]string{"postgres"}[0],
			Auth: &kindav1.DbInstanceAuth{
				Username: &kindav1.ValueSource{
					ValueFrom: &kindav1.ValueFrom{
						SecretKeyRef: &kindav1.SecretOrCMRef{
							Namespace: &namespace,
							Name:      &secretName,
							Key:       &[]string{"username"}[0],
						},
					},
				},
				Password: &kindav1.ValueSource{
					ValueFrom: &kindav1.ValueFrom{
						SecretKeyRef: &kindav1.SecretOrCMRef{
							Namespace: &namespace,
							Name:      &secretName,
							Key:       &[]string{"password"}[0],
						},
					},
				},
			},
			Endpoint: &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{
					ValueFrom: &kindav1.ValueFrom{
						ConfigMapKeyRef: &kindav1.SecretOrCMRef{
							Namespace: &namespace,
							Name:      &configMapName,
							Key:       &[]string{"host"}[0],
						},
					},
				},
				Port: &kindav1.ValueSource{
					ValueFrom: &kindav1.ValueFrom{
						ConfigMapKeyRef: &kindav1.SecretOrCMRef{
							Namespace: &namespace,
							Name:      &configMapName,
							Key:       &[]string{"port"}[0],
						},
					},
				},
			},
		},
	}

	r := func() *controller.DbInstanceReconciler {
		return &controller.DbInstanceReconciler{Client: k8sClient, Interval: interval, Conf: conf}
	}

	Context("Non-existent resource", func() {
		It("Checks the non-existent resource", func() {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: dbinstance.Name + "dummy",
				},
			}
			res, err := r().Reconcile(context.Background(), req)
			assert.NoError(GinkgoT(), err)
			assert.Equal(GinkgoT(), res, reconcile.Result{})
		})
	})

	Context("Successful reconciliation loop", func() {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: dbinstance.Name,
			},
		}
		var currentExpectedTTL time.Time

		It("Prepare resources", Serial, func() {
			ctx := GinkgoT().Context()
			assert.NoError(GinkgoT(), k8sClient.Create(ctx, secret))
			assert.NoError(GinkgoT(), k8sClient.Create(ctx, configMap))
			assert.NoError(GinkgoT(), k8sClient.Create(ctx, dbinstance))
		})

		It("Checks the full reconciliation without errors", Serial, func() {
			ctx := GinkgoT().Context()
			res, err := r().Reconcile(ctx, req)
			// Getting a new dbinstance resource
			dbin := &kindav1.DbInstance{}
			assert.NoError(GinkgoT(), k8sClient.Get(GinkgoT().Context(), req.NamespacedName, dbin))

			assert.NoError(GinkgoT(), err)
			assert.Equal(GinkgoT(), "postgres", dbin.Status.Engine)

			assert.Equal(GinkgoT(), dbin.Status.OperatorVersion, commonhelper.OperatorVersion)
			assert.Equal(GinkgoT(), res, reconcile.Result{RequeueAfter: interval})
			assert.Equal(GinkgoT(), dbin.Status.MainEndpoint.Host, test.GetPostgresHost())
			assert.Equal(GinkgoT(), dbin.Status.MainEndpoint.Port, test.GetPostgresPort())

			assert.True(GinkgoT(), dbin.Status.Ready)
			assert.Len(GinkgoT(), dbin.Status.WatchedResources, 2)

			currentExpectedTTL = time.Now().Add(conf.ServerVersionTTL).Truncate(time.Second)
			assert.NotEmpty(GinkgoT(), dbin.Status.Version)
			assert.GreaterOrEqual(GinkgoT(), time.Second*2, currentExpectedTTL.Sub(time.Unix(dbin.Status.VersionTTL, 0)).Abs())
		})

		It("Is not able to fetch user credentials", Serial, func() {
			ctx := GinkgoT().Context()
			res, err := r().Reconcile(ctx, req)
			// Getting a new dbinstance resource
			dbin := &kindav1.DbInstance{}
			assert.NoError(GinkgoT(), k8sClient.Get(GinkgoT().Context(), req.NamespacedName, dbin))

			assert.NoError(GinkgoT(), err)
			assert.Equal(GinkgoT(), "postgres", dbin.Status.Engine)

			assert.Equal(GinkgoT(), dbin.Status.OperatorVersion, commonhelper.OperatorVersion)
			assert.Equal(GinkgoT(), res, reconcile.Result{RequeueAfter: interval})
			assert.Equal(GinkgoT(), dbin.Status.MainEndpoint.Host, test.GetPostgresHost())
			assert.Equal(GinkgoT(), dbin.Status.MainEndpoint.Port, test.GetPostgresPort())

			assert.True(GinkgoT(), dbin.Status.Ready)
			assert.Len(GinkgoT(), dbin.Status.WatchedResources, 2)

			currentExpectedTTL = time.Now().Add(conf.ServerVersionTTL).Truncate(time.Second)
			assert.NotEmpty(GinkgoT(), dbin.Status.Version)
			assert.GreaterOrEqual(GinkgoT(), time.Second*2, currentExpectedTTL.Sub(time.Unix(dbin.Status.VersionTTL, 0)).Abs())
		})

		It("Checks VersionTTL logic", Serial, func() {
			ctx := GinkgoT().Context()
			// Run another reconcile and expect to have the same TTL as before
			_, err := r().Reconcile(ctx, req)
			assert.NoError(GinkgoT(), err)
			dbin := &kindav1.DbInstance{}
			assert.NoError(GinkgoT(), k8sClient.Get(GinkgoT().Context(), req.NamespacedName, dbin))
			GinkgoT().Logf("currentExpectedTTL: %v", currentExpectedTTL)
			assert.GreaterOrEqual(GinkgoT(), time.Second*2, currentExpectedTTL.Sub(time.Unix(dbin.Status.VersionTTL, 0)).Abs())

			time.Sleep(conf.ServerVersionTTL)
			_, err = r().Reconcile(ctx, req)
			assert.NoError(GinkgoT(), err)
			// After sleeping it should not equal old TTL anymore
			assert.NoError(GinkgoT(), k8sClient.Get(GinkgoT().Context(), req.NamespacedName, dbin))

			assert.LessOrEqual(GinkgoT(), time.Second*2, currentExpectedTTL.Sub(time.Unix(dbin.Status.VersionTTL, 0)).Abs())
			currentExpectedTTL = time.Now().Add(conf.ServerVersionTTL).Truncate(time.Second)
			assert.GreaterOrEqual(GinkgoT(), time.Second*2, currentExpectedTTL.Sub(time.Unix(dbin.Status.VersionTTL, 0)).Abs())
		})
	})
})
