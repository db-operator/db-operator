package v1beta1_test

import (
	"testing"

	"github.com/db-operator/db-operator/api/v1beta1"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/stretchr/testify/assert"
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
	from := &v1beta1.FromRef{
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
	from := &v1beta1.FromRef{
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
	from := &v1beta1.FromRef{
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
		HostFrom: &v1beta1.FromRef{
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
		HostFrom: &v1beta1.FromRef{
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
		PortFrom: &v1beta1.FromRef{
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
		PortFrom: &v1beta1.FromRef{
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
		PublicIPFrom: &v1beta1.FromRef{
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
		PublicIPFrom: &v1beta1.FromRef{
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
