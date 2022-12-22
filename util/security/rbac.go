package security

import (
	"fmt"
)

// AppRBACName constructs name of the app for use in RBAC checks.
func AppRBACName(defaultNS string, project string, namespace string, name string) string {
	if defaultNS != "" && namespace != defaultNS && namespace != "" {
		return fmt.Sprintf("%s/%s/%s", project, namespace, name)
	} else {
		return fmt.Sprintf("%s/%s", project, name)
	}
}
