package helpers

import (
	"regexp"
	"slices"

	kindav1 "github.com/db-operator/db-operator/v2/api/v1"
)

// CheckAllowedPrivileges verifies whether a privilege can be assigned to a DbUser
func CheckAllowedPrivileges(role string, namespace string, allowedPrivileges []kindav1.DbInstanceAllowedPrivileges) (bool, error) {
	for _, rule := range allowedPrivileges {
		re, err := regexp.Compile(rule.NamespaceRegex)
		if err != nil {
			return false, err
		}

		if re.MatchString(namespace) {
			if slices.Contains(rule.Roles, role) {
				return true, nil
			}
		}
	}

	return false, nil
}
