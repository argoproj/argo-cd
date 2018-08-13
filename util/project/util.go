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

// GetJWTTokenIndexByIssuedAt looks up the index of a JWTToken in a project by the issue at time
func GetJWTTokenIndexByIssuedAt(proj *v1alpha1.AppProject, roleIndex int, issuedAt int64) (int, error) {
	for i, token := range proj.Spec.Roles[roleIndex].JWTTokens {
		if issuedAt == token.IssuedAt {
			return i, nil
		}
	}
	return -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", proj.Spec.Roles[roleIndex].Name, issuedAt, proj.Name)
}
