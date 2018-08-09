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

// GetJwtTokenIndexByCreatedAt looks up the index of a JwtToken in a project by the created at time
func GetJwtTokenIndexByCreatedAt(proj *v1alpha1.AppProject, roleIndex int, createdAt int64) (int, error) {
	for i, token := range proj.Spec.Roles[roleIndex].JwtTokens {
		if createdAt == token.CreatedAt {
			return i, nil
		}
	}
	return -1, fmt.Errorf("JwtToken for role '%s' with '%d' created time does not exist in project '%s'", proj.Spec.Roles[roleIndex].Name, createdAt, proj.Name)
}
