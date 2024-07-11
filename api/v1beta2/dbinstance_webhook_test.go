package v1beta2_test

import (
	"testing"

	"github.com/db-operator/db-operator/api/common"
	"github.com/db-operator/db-operator/api/v1beta2"
	"github.com/db-operator/db-operator/pkg/consts"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnitEngineValid(t *testing.T) {
	err := v1beta2.ValidateEngine("postgres")
	assert.NoError(t, err)

	err = v1beta2.ValidateEngine("mysql")
	assert.NoError(t, err)
}

func TestUnitEngineInvalid(t *testing.T) {
	err := v1beta2.ValidateEngine("dummy")
	assert.Error(t, err)
}

func TestValidateFromSecret(t *testing.T) {
	from := &common.FromRef{
		Kind: "Secret",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta2.InstanceData{
		HostFrom: from,
		PortFrom: from,
	}
	assert.NoError(t, v1beta2.ValidateConfigFrom(dbin))
}

func TestValidateFromCM(t *testing.T) {
	from := &common.FromRef{
		Kind: "ConfigMap",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta2.InstanceData{
		HostFrom: from,
		PortFrom: from,
	}
	assert.NoError(t, v1beta2.ValidateConfigFrom(dbin))
}

func TestValidateFromUnknown(t *testing.T) {
	from := &common.FromRef{
		Kind: "dummy",
		Name: "name",
		Key:  "key",
	}
	dbin := &v1beta2.InstanceData{
		HostFrom: from,
		PortFrom: from,
	}
	assert.Error(t, v1beta2.ValidateConfigFrom(dbin))
}

func TestUnitConfigHostErr(t *testing.T) {
	spec := &v1beta2.InstanceData{
		Host: "host",
		HostFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.Error(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigHostVal(t *testing.T) {
	spec := &v1beta2.InstanceData{
		Host: "host",
	}
	assert.NoError(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigHostFrom(t *testing.T) {
	spec := &v1beta2.InstanceData{
		HostFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.NoError(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortErr(t *testing.T) {
	spec := &v1beta2.InstanceData{
		Port: 5432,
		PortFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.Error(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortVal(t *testing.T) {
	spec := &v1beta2.InstanceData{
		Port: 5432,
	}
	assert.NoError(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestUnitConfigPortFrom(t *testing.T) {
	spec := &v1beta2.InstanceData{
		PortFrom: &common.FromRef{
			Kind: "ConfigMap",
			Name: "name",
			Key:  "key",
		},
	}
	assert.NoError(t, v1beta2.ValidateConfigVsConfigFrom(spec))
}

func TestAllowedPrivilegesFail1(t *testing.T) {
	privileges := []string{consts.ALL_PRIVILEGES}
	err := v1beta2.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivilegesFail2(t *testing.T) {
	privileges := []string{"all privileges"}
	err := v1beta2.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivilegesFail3(t *testing.T) {
	privileges := []string{"aLL PriVileges"}
	err := v1beta2.TestAllowedPrivileges(privileges)
	assert.ErrorContains(t, err, "it's not allowed to grant ALL PRIVILEGES")
}

func TestAllowedPrivileges(t *testing.T) {
	privileges := []string{"rds_admin"}
	err := v1beta2.TestAllowedPrivileges(privileges)
	assert.NoError(t, err)
}

func TestUpdateNewInstanceDataErr(t *testing.T) {
	oldDbin := v1beta2.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "name",
		},
		Spec: v1beta2.DbInstanceSpec{
			InstanceData: &v1beta2.InstanceData{
				Host: "localhost",
				Port: 3306,
			},
		},
		Status: v1beta2.DbInstanceStatus{
			Connected: true,
		},
	}
	new := oldDbin.DeepCopy()
	new.Spec.InstanceData = &v1beta2.InstanceData{
		Host: "0.0.0.0",
		Port: 3304,
	}
	_, err := new.ValidateUpdate(&oldDbin)
	assert.ErrorContains(t, err, "set the kinda.rocks/db-instance-allow-migration annotation to 'true'")
}

func TestUpdateNewInstanceNotConnected(t *testing.T) {
	oldDbin := v1beta2.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "name",
		},
		Spec: v1beta2.DbInstanceSpec{
			InstanceData: &v1beta2.InstanceData{
				Host: "localhost",
				Port: 3306,
			},
		},
	}
	new := oldDbin.DeepCopy()
	new.Spec.InstanceData = &v1beta2.InstanceData{
		Host: "0.0.0.0",
		Port: 3304,
	}
	_, err := new.ValidateUpdate(&oldDbin)
	assert.NoError(t, err)
}

func TestUpdateNewInstanceAnnotation(t *testing.T) {
	oldDbin := v1beta2.DbInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "name",
			Annotations: map[string]string{
				consts.DBINSTANCE_ALLOW_MIGRATION: "true",
			},
		},
		Spec: v1beta2.DbInstanceSpec{
			InstanceData: &v1beta2.InstanceData{
				Host: "localhost",
				Port: 3306,
			},
		},
		Status: v1beta2.DbInstanceStatus{
			Connected: true,
		},
	}
	new := oldDbin.DeepCopy()
	new.Spec.InstanceData = &v1beta2.InstanceData{
		Host: "0.0.0.0",
		Port: 3304,
	}
	_, err := new.ValidateUpdate(&oldDbin)
	assert.NoError(t, err)
}
