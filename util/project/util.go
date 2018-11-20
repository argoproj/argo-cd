package project

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetRoleByName returns the role in a project by the name with its index
func GetRoleByName(proj *v1alpha1.AppProject, name string) (*v1alpha1.ProjectRole, int, error) {
	for i, role := range proj.Spec.Roles {
		if name == role.Name {
			return &role, i, nil
		}
	}
	return nil, -1, fmt.Errorf("role '%s' does not exist in project '%s'", name, proj.Name)
}

// GetJWTToken looks up the index of a JWTToken in a project by the issue at time
func GetJWTToken(proj *v1alpha1.AppProject, roleName string, issuedAt int64) (*v1alpha1.JWTToken, int, error) {
	role, _, err := GetRoleByName(proj, roleName)
	if err != nil {
		return nil, -1, err
	}
	for i, token := range role.JWTTokens {
		if issuedAt == token.IssuedAt {
			return &token, i, nil
		}
	}
	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", role.Name, issuedAt, proj.Name)
}

func ValidateProject(p *v1alpha1.AppProject) error {
	destKeys := make(map[string]bool)
	for _, dest := range p.Spec.Destinations {
		key := fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)
		if _, ok := destKeys[key]; !ok {
			destKeys[key] = true
		} else {
			return status.Errorf(codes.InvalidArgument, "destination %s should not be listed more than once.", key)
		}
	}
	srcRepos := make(map[string]bool)
	for i, src := range p.Spec.SourceRepos {
		if src != "*" {
			src = git.NormalizeGitURL(src)
		}
		p.Spec.SourceRepos[i] = src
		if _, ok := srcRepos[src]; !ok {
			srcRepos[src] = true
		} else {
			return status.Errorf(codes.InvalidArgument, "source repository %s should not be listed more than once.", src)
		}
	}

	roleNames := make(map[string]bool)
	for _, role := range p.Spec.Roles {
		existingPolicies := make(map[string]bool)
		for _, policy := range role.Policies {
			var err error
			if role.JWTTokens != nil {
				err = validateJWTToken(p.Name, role.Name, policy)
			} else {
				err = validatePolicy(p.Name, policy)
			}
			if err != nil {
				return err
			}
			if _, ok := existingPolicies[policy]; !ok {
				existingPolicies[policy] = true
			} else {
				return status.Errorf(codes.AlreadyExists, "policy '%s' already exists for role '%s'", policy, role.Name)
			}
		}
		if _, ok := roleNames[role.Name]; !ok {
			roleNames[role.Name] = true
		} else {
			return status.Errorf(codes.AlreadyExists, "can't have duplicate roles: role '%s' already exists", role.Name)
		}
	}

	return nil
}

func validateJWTToken(proj string, token string, policy string) error {
	err := validatePolicy(proj, policy)
	if err != nil {
		return err
	}
	policyComponents := strings.Split(policy, ",")
	if strings.Trim(policyComponents[2], " ") != "applications" {
		return status.Errorf(codes.InvalidArgument, "incorrect format for '%s' as JWT tokens can only access applications", policy)
	}
	roleComponents := strings.Split(strings.Trim(policyComponents[1], " "), ":")
	if len(roleComponents) != 3 {
		return status.Errorf(codes.InvalidArgument, "incorrect number of role arguments for '%s' policy", policy)
	}
	if roleComponents[0] != "proj" {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as role should start with 'proj:'", policy)
	}
	if roleComponents[1] != proj {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as policy can't grant access to other projects", policy)
	}
	if roleComponents[2] != token {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as policy can't grant access to other roles", policy)
	}
	return nil
}

func validatePolicy(proj string, policy string) error {
	policyComponents := strings.Split(policy, ",")
	if len(policyComponents) != 6 {
		return status.Errorf(codes.InvalidArgument, "incorrect number of policy arguments for '%s'", policy)
	}
	if strings.Trim(policyComponents[0], " ") != "p" {
		return status.Errorf(codes.InvalidArgument, "policies can only use the policy format: '%s'", policy)
	}
	if len(strings.Trim(policyComponents[1], " ")) <= 0 {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as subject must be longer than 0 characters:", policy)
	}
	if len(strings.Trim(policyComponents[2], " ")) <= 0 {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as object must be longer than 0 characters:", policy)
	}
	if len(strings.Trim(policyComponents[3], " ")) <= 0 {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as action must be longer than 0 characters:", policy)
	}
	if !strings.HasPrefix(strings.Trim(policyComponents[4], " "), proj) {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as policies can't grant access to other projects", policy)
	}
	effect := strings.Trim(policyComponents[5], " ")
	if effect != "allow" && effect != "deny" {
		return status.Errorf(codes.InvalidArgument, "incorrect policy format for '%s' as effect can only have value 'allow' or 'deny'", policy)
	}
	return nil
}

// AddGroupToRole adds an OIDC group to a role
func AddGroupToRole(p *v1alpha1.AppProject, roleName, group string) (bool, error) {
	role, roleIndex, err := GetRoleByName(p, roleName)
	if err != nil {
		return false, err
	}
	for _, group := range role.Groups {
		if group == group {
			return false, nil
		}
	}
	role.Groups = append(role.Groups, group)
	p.Spec.Roles[roleIndex] = *role
	return true, nil
}

// RemoveGroupFromRole removes an OIDC group from a role
func RemoveGroupFromRole(p *v1alpha1.AppProject, roleName, group string) (bool, error) {
	role, roleIndex, err := GetRoleByName(p, roleName)
	if err != nil {
		return false, err
	}
	for i, group := range role.Groups {
		if group == group {
			role.Groups = append(role.Groups[0:i], role.Groups[i:]...)
			p.Spec.Roles[roleIndex] = *role
			return true, nil
		}
	}
	return false, nil
}
