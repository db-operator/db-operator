package helpers_test

import (
	"testing"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
	"github.com/db-operator/db-operator/v2/internal/controller/helpers"
	"github.com/stretchr/testify/assert"
)

func TestUnitCheckAllowedPrivileges(t *testing.T) {
	t.Parallel()
	allowedPrivileges := []kindav1.DbInstanceAllowedPrivileges{
		{NamespaceRegex: "default", Roles: []string{"readOnly"}},
		{NamespaceRegex: "test-*", Roles: []string{"readWrite"}},
	}
	t.Run("Allowed simple string", func(t *testing.T) {
		allowed, err := helpers.CheckAllowedPrivileges("readOnly", "default", allowedPrivileges)
		assert.NoError(t, err)
		assert.True(t, allowed)
	})
	t.Run("Not allowed simple string", func(t *testing.T) {
		allowed, err := helpers.CheckAllowedPrivileges("readOnly", "test-1", allowedPrivileges)
		assert.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("Allowed regex", func(t *testing.T) {
		allowed, err := helpers.CheckAllowedPrivileges("readWrite", "test-1", allowedPrivileges)
		assert.NoError(t, err)
		assert.True(t, allowed)
	})
	t.Run("Not allowed simple regex", func(t *testing.T) {
		allowed, err := helpers.CheckAllowedPrivileges("readWrite", "default", allowedPrivileges)
		assert.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("Not allowed simple regex one more time", func(t *testing.T) {
		allowed, err := helpers.CheckAllowedPrivileges("readWrite", "check", allowedPrivileges)
		assert.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("Invalid regex", func(t *testing.T) {
		allowedPrivileges := []kindav1.DbInstanceAllowedPrivileges{
			{NamespaceRegex: "(abv", Roles: []string{"readOnly"}},
		}
		allowed, err := helpers.CheckAllowedPrivileges("readWrite", "check", allowedPrivileges)
		assert.Error(t, err)
		assert.False(t, allowed)
	})
}
