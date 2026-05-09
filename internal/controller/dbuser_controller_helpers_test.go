package controllers

import (
	"testing"

	kindav1beta2 "github.com/db-operator/db-operator/v2/api/v1beta2"
	commonhelper "github.com/db-operator/db-operator/v2/internal/helpers/common"
	"github.com/db-operator/db-operator/v2/pkg/utils/kci"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseDbUserSecretDataPostgres(t *testing.T) {
	data := map[string][]byte{
		"POSTGRES_DB":       []byte("appdb"),
		"POSTGRES_USER":     []byte("appuser"),
		"POSTGRES_PASSWORD": []byte("apppass"),
	}

	cred, err := parseDbUserSecretData("postgres", data)
	assert.NoError(t, err)
	assert.Equal(t, "appdb", cred.Name)
	assert.Equal(t, "appuser", cred.Username)
	assert.Equal(t, "apppass", cred.Password)
}

func TestParseDbUserSecretDataMysql(t *testing.T) {
	data := map[string][]byte{
		"DB":       []byte("appdb"),
		"USER":     []byte("appuser"),
		"PASSWORD": []byte("apppass"),
	}

	cred, err := parseDbUserSecretData("mysql", data)
	assert.NoError(t, err)
	assert.Equal(t, "appdb", cred.Name)
	assert.Equal(t, "appuser", cred.Username)
	assert.Equal(t, "apppass", cred.Password)
}

func TestParseDbUserSecretDataErrors(t *testing.T) {
	_, err := parseDbUserSecretData("postgres", map[string][]byte{})
	assert.ErrorContains(t, err, "POSTGRES_DB key does not exist")

	_, err = parseDbUserSecretData("mysql", map[string][]byte{})
	assert.ErrorContains(t, err, "DB key does not exist")

	_, err = parseDbUserSecretData("oracle", map[string][]byte{})
	assert.ErrorContains(t, err, "not supported engine type")
}

func TestIsDbUserChanged(t *testing.T) {
	dbu := &kindav1beta2.DbUser{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: kindav1beta2.DbUserSpec{
			DatabaseRef: "mydb",
			AccessType:  kindav1beta2.READWRITE,
		},
	}
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"POSTGRES_DB":       []byte("mydb"),
			"POSTGRES_USER":     []byte("myuser"),
			"POSTGRES_PASSWORD": []byte("mypassword"),
		},
	}

	specChecksum := kci.GenerateChecksum(dbu.Spec)
	secretChecksum := commonhelper.GenerateChecksumSecretValue(secret)
	dbu.SetAnnotations(map[string]string{
		"checksum/spec":   specChecksum,
		"checksum/secret": secretChecksum,
	})

	assert.False(t, isDbUserChanged(dbu, secret))

	dbu.SetAnnotations(map[string]string{
		"checksum/spec":   "different",
		"checksum/secret": secretChecksum,
	})
	assert.True(t, isDbUserChanged(dbu, secret))

	dbu.SetAnnotations(map[string]string{
		"checksum/spec":   specChecksum,
		"checksum/secret": "different",
	})
	assert.True(t, isDbUserChanged(dbu, secret))
}

func TestIsDbUserChangedWithoutAnnotations(t *testing.T) {
	dbu := &kindav1beta2.DbUser{
		Spec: kindav1beta2.DbUserSpec{
			DatabaseRef: "mydb",
			AccessType:  kindav1beta2.READWRITE,
		},
	}
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"POSTGRES_DB":       []byte("mydb"),
			"POSTGRES_USER":     []byte("myuser"),
			"POSTGRES_PASSWORD": []byte("mypassword"),
		},
	}

	assert.True(t, isDbUserChanged(dbu, secret))
}
