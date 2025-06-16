package security

import (
	"fmt"
)

// RBACName constructs name of the app for use in RBAC checks.
func RBACName(defaultNS, project, namespace, name string) string {
	if defaultNS != "" && namespace != defaultNS && namespace != "" {
		return fmt.Sprintf("%s/%s/%s", project, namespace, name)
	}
	return fmt.Sprintf("%s/%s", project, name)
}
