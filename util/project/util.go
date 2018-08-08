package project

import "fmt"
import "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

// GetRoleIndexByName looks up the index of a role in a project by the name
func GetRoleIndexByName(proj *v1alpha1.AppProject, name string) (int, error) {
	for i, role := range proj.Spec.Roles {
		if name == role.Name {
			return i, nil
		}
	}
	return -1, fmt.Errorf("role '%s' does not exist in project '%s'", name, proj.Name)
}
