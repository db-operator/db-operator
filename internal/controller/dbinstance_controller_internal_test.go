package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	"github.com/db-operator/db-operator/v2/pkg/consts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestUnitFindDbInstanceForResource(t *testing.T) {
	t.Parallel()
	r := DbInstanceReconciler{}
	t.Run("Label found", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
				Labels: map[string]string{
					consts.DBINSTANCE_NAME_LABEL_KEY: "test-dbinstance",
				},
			},
		}

		result := r.findDbInstanceForResource(t.Context(), secret)
		assert.Len(t, result, 1)
		assert.Equal(t, "test-dbinstance", result[0].Name)
	})
	t.Run("Label not found", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-secret",
			},
		}

		result := r.findDbInstanceForResource(t.Context(), secret)
		assert.Nil(t, result)
	})
}

func TestUnitReconcilerSuite(t *testing.T) {
	namespace := "default"

	secretName := "my-secret"
	secretKey := "password"
	secretValue := "qwertyu9"

	configMapName := "my-config"
	configMapNameStale := "my-config-stale"

	configMapKey := "user"
	configMapValue := "db-operator"

	expectedErrUpdate := errors.New("update failed")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretKey: []byte(secretValue),
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
			configMapKey: configMapValue,
		},
	}

	configMapStale := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapNameStale,
			Namespace: namespace,
			Labels: map[string]string{
				consts.DBINSTANCE_NAME_LABEL_KEY: "test-dbinstance",
			},
		},
		Data: map[string]string{
			configMapKey: configMapValue,
		},
	}
	t.Parallel()

	dbinstance := &kindav1.DbInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dbinstance",
		},
		Spec: kindav1.DbInstanceSpec{},
	}
	r := func() *DbInstanceReconciler {
		clientOk := fake.NewClientBuilder().
			WithObjects(secret.DeepCopy(), configMap.DeepCopy(), configMapStale.DeepCopy()).
			Build()
		return &DbInstanceReconciler{Client: clientOk}
	}

	rWithErr := func() *DbInstanceReconciler {
		clientErr := fake.NewClientBuilder().
			WithObjects(secret.DeepCopy(), configMap.DeepCopy(), configMapStale.DeepCopy()).
			WithInterceptorFuncs(interceptor.Funcs{
				Update: func(
					ctx context.Context,
					c client.WithWatch,
					obj client.Object,
					opts ...client.UpdateOption,
				) error {
					return expectedErrUpdate
				},
			}).
			Build()
		return &DbInstanceReconciler{Client: clientErr}
	}
	t.Run("Fetch endpoint tests", func(t *testing.T) {
		host := "localhost"
		port := "5432"
		engine := "postgres"
		t.Run("Empty host error", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{},
			}
			db, err := r().fetchEndpoint(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "failed to fetch host")
			assert.Nil(t, db)
		})
		t.Run("Empty port error", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{Value: &host},
				Port: &kindav1.ValueSource{},
			}
			db, err := r().fetchEndpoint(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "failed to fetch port")
			assert.Nil(t, db)
		})
		t.Run("Invalid port error", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			invalidPort := "test"
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{Value: &host},
				Port: &kindav1.ValueSource{Value: &invalidPort},
			}
			db, err := r().fetchEndpoint(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "failed to convert port")
			assert.Nil(t, db)
		})

		t.Run("SSL Connection is nil", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{
				Host: &kindav1.ValueSource{Value: &host},
				Port: &kindav1.ValueSource{Value: &port},
			}
			dbinstance.Spec.Engine = &engine
			db, err := r().fetchEndpoint(t.Context(), dbinstance)
			assert.NoError(t, err)
			assert.Equal(t, engine, db.Engine)
			assert.False(t, db.SSLEnabled)
			assert.False(t, db.SkipCAVerify)
			assert.Equal(t, host, db.Host)
			assert.Equal(t, uint16(5432), db.Port)
		})

		t.Run("SSL Connection is set", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			sslConnection := &kindav1.DbInstanceSSLConnection{true, true}
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{
				Host:          &kindav1.ValueSource{Value: &host},
				Port:          &kindav1.ValueSource{Value: &port},
				SSLConnection: sslConnection,
			}
			dbinstance.Spec.Engine = &engine
			db, err := r().fetchEndpoint(t.Context(), dbinstance)
			assert.NoError(t, err)
			assert.Equal(t, engine, db.Engine)
			assert.True(t, db.SSLEnabled)
			assert.True(t, db.SkipCAVerify)
			assert.Equal(t, host, db.Host)
			assert.Equal(t, uint16(5432), db.Port)
		})
	})

	t.Run("Fetch credentials tests", func(t *testing.T) {
		password := "qwertyu9"
		username := "db-operator"
		t.Run("Empty username error", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Auth = &kindav1.DbInstanceAuth{}
			dbinstance.Spec.Auth.Password = &kindav1.ValueSource{
				Value: &password,
			}
			dbuser, err := r().fetchCredentials(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "failed to fetch username from source")
			assert.Nil(t, dbuser)
		})
		t.Run("Empty password error", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Auth = &kindav1.DbInstanceAuth{}
			dbinstance.Spec.Auth.Username = &kindav1.ValueSource{
				Value: &username,
			}
			dbuser, err := r().fetchCredentials(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "failed to fetch password from source")
			assert.Nil(t, dbuser)
		})
		t.Run("Success", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Auth = &kindav1.DbInstanceAuth{
				Username: &kindav1.ValueSource{
					Value: &username,
				},
				Password: &kindav1.ValueSource{
					Value: &password,
				},
			}
			dbuser, err := r().fetchCredentials(t.Context(), dbinstance)
			assert.NoError(t, err)
			assert.Equal(t, password, dbuser.Password)
			assert.Equal(t, username, dbuser.Username)
		})
	})
	t.Run("Label resources", func(t *testing.T) {
		t.Parallel()
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

		// Add the missing Status field to avoid nil pointer dereference
		dbinstance.Status = kindav1.DbInstanceStatus{
			Engine: "postgres",
			ServerStatus: &kindav1.DbInstanceServerStatus{
				DatabasesCount: 0,
				Users:          []string{},
			},
		}

		// Add the missing namespace filters to avoid nil pointer dereference
		if dbinstance.Spec.NamespaceFilters == nil {
			dbinstance.Spec.NamespaceFilters = []string{}
		}

		// Add the missing grant rules to avoid nil pointer dereference
		if dbinstance.Spec.GrantRules == nil {
			dbinstance.Spec.GrantRules = []*kindav1.DbInstanceGrantRule{}
		}

		// Test with valid data
		t.Run("Valid Data", func(t *testing.T) {
			err := r().labelReferencedResources(t.Context(), dbinstance.DeepCopy())

			assert.NoError(t, err)
		})

		// Test with nil auth
		t.Run("Nil Auth", func(t *testing.T) {
			dbinstanceNilAuth := dbinstance.DeepCopy()
			dbinstanceNilAuth.Spec.Auth = nil

			err := r().labelReferencedResources(t.Context(), dbinstanceNilAuth)

			assert.Error(t, err)
			assert.ErrorContains(t, err, "auth data is nil")
		})

		// Test with nil endpoint
		t.Run("Nil Endpoint", func(t *testing.T) {
			dbinstanceNilEndpoint := dbinstance.DeepCopy()
			dbinstanceNilEndpoint.Spec.Endpoint = nil

			err := r().labelReferencedResources(t.Context(), dbinstanceNilEndpoint)

			assert.Error(t, err)
			assert.ErrorContains(t, err, "endpoint data is nil")
		})

		// Test with nil username
		t.Run("Nil Username", func(t *testing.T) {
			dbinstanceNilUsername := dbinstance.DeepCopy()
			dbinstanceNilUsername.Spec.Auth.Username = nil
			err := r().labelReferencedResources(t.Context(), dbinstanceNilUsername)

			assert.NoError(t, err)
		})

		// Test with nil password
		t.Run("Nil Password", func(t *testing.T) {
			dbinstanceNilPassword := dbinstance.DeepCopy()
			dbinstanceNilPassword.Spec.Auth.Password = nil

			err := r().labelReferencedResources(t.Context(), dbinstanceNilPassword)

			assert.NoError(t, err)
		})

		// Test with nil host
		t.Run("Nil Host", func(t *testing.T) {
			dbinstanceNilHost := dbinstance.DeepCopy()
			dbinstanceNilHost.Spec.Endpoint.Host = nil

			err := r().labelReferencedResources(t.Context(), dbinstanceNilHost)

			assert.NoError(t, err)
		})

		// Test with nil port
		t.Run("Nil Port", func(t *testing.T) {
			dbinstanceNilPort := dbinstance.DeepCopy()
			dbinstanceNilPort.Spec.Endpoint.Port = nil

			err := r().labelReferencedResources(t.Context(), dbinstanceNilPort)

			assert.NoError(t, err)
		})

		// Test with non-existent secret
		t.Run("Non-existent Secret", func(t *testing.T) {
			dbinstanceBadSecret := dbinstance.DeepCopy()
			dbinstanceBadSecret.Spec.Auth.Username.ValueFrom.SecretKeyRef.Name = &[]string{"non-existent"}[0]

			err := r().labelReferencedResources(t.Context(), dbinstanceBadSecret)

			assert.Error(t, err)
			assert.ErrorContains(t, err, "not found")
		})

		t.Run("Error updating a resource", func(t *testing.T) {
			dbinstanceBadSecret := dbinstance.DeepCopy()
			err := rWithErr().labelReferencedResources(t.Context(), dbinstanceBadSecret)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expectedErrUpdate)
		})

		t.Run("Stale resource detection", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Status.WatchedResources = []string{fmt.Sprintf("%s/%s/%s", configMapStale.Kind, configMapStale.Namespace, configMapStale.Name)}
			err := r().labelReferencedResources(t.Context(), dbinstance)
			assert.NoError(t, err)
		})

		t.Run("Stale resource detection wrong kind", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Status.WatchedResources = []string{fmt.Sprintf("WrongKind/%s/%s", configMapStale.Namespace, configMapStale.Name)}
			err := r().labelReferencedResources(t.Context(), dbinstance)
			assert.Error(t, err)
		})

		t.Run("Stale resource detection wrong kind", func(t *testing.T) {
			dbinstance := dbinstance.DeepCopy()
			dbinstance.Spec.Auth = &kindav1.DbInstanceAuth{}
			dbinstance.Spec.Endpoint = &kindav1.DbInstanceEndpoint{}
			dbinstance.Status.WatchedResources = []string{fmt.Sprintf("%s/%s/%s", configMapStale.Kind, configMapStale.Namespace, configMapStale.Name)}
			err := rWithErr().labelReferencedResources(t.Context(), dbinstance)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expectedErrUpdate)
		})
	})
}
