package security

import (
	"fmt"
)

// RBACName constructs name of the app for use in RBAC checks.
func RBACName(defaultNS string, project string, namespace string, name string) string {
	if namespace == "" {
		namespace = defaultNS
	}
	return fmt.Sprintf("%s/%s/%s", project, namespace, name)
}
