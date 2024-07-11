package v1beta1_test

import (
	"testing"

	"github.com/db-operator/db-operator/api/common"
	"github.com/db-operator/db-operator/api/v1beta1"
	"github.com/db-operator/db-operator/api/v1beta2"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnitEngineValid(t *testing.T) {
	err := v1beta1.ValidateEngine("postgres")
	assert.NoError(t, err)

	err = v1beta1.ValidateEngine("mysql")
	assert.NoError(t, err)
}

func TestUnitEngineInvalid(t *testing.T) {
	err := v1beta1.ValidateEngine("dummy")
	assert.Error(t, err)
}

func TestValidateFromSecret(t *testing.T) {
	from := &common.FromRef{
		Kind: "Secret",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta1.GenericInstance{
		HostFrom:     from,
		PortFrom:     from,
		PublicIPFrom: from,
	}
	assert.NoError(t, v1beta1.ValidateConfigFrom(dbin))
}

func TestValidateFromCM(t *testing.T) {
	from := &common.FromRef{
		Kind: "ConfigMap",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta1.GenericInstance{
		HostFrom:     from,
		PortFrom:     from,
		PublicIPFrom: from,
	}
	assert.NoError(t, v1beta1.ValidateConfigFrom(dbin))
}

func TestValidateFromUnknown(t *testing.T) {
	from := &common.FromRef{
		Kind: "dummy",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta1.GenericInstance{
		HostFrom:     from,
		PortFrom:     from,
		PublicIPFrom: from,
	}
	assert.Error(t, v1beta1.ValidateConfigFrom(dbin))
}

func TestUnitConfigHostErr(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		Host: "host",
		HostFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.Error(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigHostVal(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		Host: "host",
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigHostFrom(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		HostFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortErr(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		Port: 5432,
		PortFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.Error(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortVal(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		Port: 5432,
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortFrom(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		PortFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPublicIPErr(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		PublicIP: "123.123.123.123",
		PublicIPFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.Error(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPublicIPVal(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		PublicIP: "123.123.123.123",
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPublicIPFrom(t *testing.T) {
	spec := &v1beta1.GenericInstance{
		PublicIPFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.NoError(t, v1beta1.ValidateConfigVsConfigFrom(spec))
}

func TestAllowedPrivilegesFail1(t *testing.T) {
	privileges := []string{consts.ALL_PRIVILEGES}
	err := v1beta1.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivilegesFail2(t *testing.T) {
	privileges := []string{"all privileges"}
	err := v1beta1.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivilegesFail3(t *testing.T) {
	privileges := []string{"aLL PriVileges"}
	err := v1beta1.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivileges(t *testing.T) {
	privileges := []string{"rds_admin"}
	err := v1beta1.TestAllowedPrivileges(privileges)
	assert.NoError(t, err)
}

// Conversion tests

func TestConversionFrom(t *testing.T) {
	old := v1beta1.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: v1beta1.DbInstanceSpec{
			Engine: "mysql",
			AdminUserSecret: v1beta1.NamespacedName{
				Namespace: "namespace",
				Name:      "name",
			},
			Backup: v1beta1.DbInstanceBackup{
				Bucket: "bucket",
			},
			Monitoring: v1beta1.DbInstanceMonitoring{
				Enabled: true,
			},
			SSLConnection: v1beta1.DbInstanceSSLConnection{
				Enabled:    true,
				SkipVerify: true,
			},
			AllowedPrivileges: []string{"test", "test2"},
			DbInstanceSource: v1beta1.DbInstanceSource{
				Generic: &v1beta1.GenericInstance{
					Host:       "0.0.0.0",
					Port:       42,
					PublicIP:   "1.1.1.1",
					BackupHost: "1.1.1.1",
				},
			},
		},
	}
	new := &v1beta2.DbInstance{}
	assert.NoError(t, old.ConvertTo(new))
	expected := v1beta2.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: v1beta2.DbInstanceSpec{
			Engine: "mysql",
			AdminCredentials: &v1beta2.AdminCredentials{
				UsernameFrom: &common.FromRef{
					Kind:      "Secret",
					Name:      "name",
					Namespace: "namespace",
					Key:       "user",
				},
				PasswordFrom: &common.FromRef{
					Kind:      "Secret",
					Name:      "name",
					Namespace: "namespace",
					Key:       "password",
				},
			},
			SSLConnection: v1beta2.DbInstanceSSLConnection{
				Enabled:    true,
				SkipVerify: true,
			},
			AllowedPrivileges: []string{"test", "test2"},
			InstanceData: &v1beta2.InstanceData{
				Host: "0.0.0.0",
				Port: 42,
			},
		},
	}
	assert.Equal(t, expected.Spec, new.Spec)
}

func TestConversionFroGoogleErr(t *testing.T) {
	old := v1beta1.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: v1beta1.DbInstanceSpec{
			Engine: "mysql",
			AdminUserSecret: v1beta1.NamespacedName{
				Namespace: "namespace",
				Name:      "name",
			},
			Backup: v1beta1.DbInstanceBackup{
				Bucket: "bucket",
			},
			Monitoring: v1beta1.DbInstanceMonitoring{
				Enabled: true,
			},
			SSLConnection: v1beta1.DbInstanceSSLConnection{
				Enabled:    true,
				SkipVerify: true,
			},
			AllowedPrivileges: []string{"test", "test2"},
			DbInstanceSource: v1beta1.DbInstanceSource{
				Google: &v1beta1.GoogleInstance{
					InstanceName: "name",
					ConfigmapName: v1beta1.NamespacedName{
						Namespace: "namespace",
						Name:      "name",
					},
					APIEndpoint: "endpoint",
					ClientSecret: v1beta1.NamespacedName{
						Namespace: "namespace",
						Name:      "name",
					},
				},
			},
		},
	}
	new := &v1beta2.DbInstance{}
	assert.Error(t, old.ConvertTo(new))
}

func TestConversionTo(t *testing.T) {
	old := v1beta2.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: v1beta2.DbInstanceSpec{
			Engine: "mysql",
			AdminCredentials: &v1beta2.AdminCredentials{
				UsernameFrom: &common.FromRef{
					Kind:      "Secret",
					Name:      "name",
					Namespace: "namespace",
					Key:       "user",
				},
				PasswordFrom: &common.FromRef{
					Kind:      "Secret",
					Name:      "name",
					Namespace: "namespace",
					Key:       "password",
				},
			},
			SSLConnection: v1beta2.DbInstanceSSLConnection{
				Enabled:    true,
				SkipVerify: true,
			},
			AllowedPrivileges: []string{"test", "test2"},
			InstanceData: &v1beta2.InstanceData{
				Host: "0.0.0.0",
				Port: 42,
			},
		},
	}
	new := &v1beta1.DbInstance{}
	assert.NoError(t, new.ConvertFrom(&old))
	expected := v1beta1.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},

		Spec: v1beta1.DbInstanceSpec{
			Engine: "mysql",
			AdminUserSecret: v1beta1.NamespacedName{
				Namespace: "namespace",
				Name:      "name",
			},
			Backup:     v1beta1.DbInstanceBackup{},
			Monitoring: v1beta1.DbInstanceMonitoring{},
			SSLConnection: v1beta1.DbInstanceSSLConnection{
				Enabled:    true,
				SkipVerify: true,
			},
			AllowedPrivileges: []string{"test", "test2"},
			DbInstanceSource: v1beta1.DbInstanceSource{
				Generic: &v1beta1.GenericInstance{
					Host: "0.0.0.0",
					Port: 42,
				},
			},
		},
	}
	assert.Equal(t, expected.Spec, new.Spec)
}
