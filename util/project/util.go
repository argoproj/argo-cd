package project

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/rbac"
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
		if _, ok := destKeys[key]; ok {
			return status.Errorf(codes.InvalidArgument, "destination '%s' already added", key)
		}
		destKeys[key] = true
	}
	srcRepos := make(map[string]bool)
	for _, src := range p.Spec.SourceRepos {
		if _, ok := srcRepos[src]; ok {
			return status.Errorf(codes.InvalidArgument, "source repository '%s' already added", src)
		}
		srcRepos[src] = true
	}

	roleNames := make(map[string]bool)
	for _, role := range p.Spec.Roles {
		if _, ok := roleNames[role.Name]; ok {
			return status.Errorf(codes.AlreadyExists, "role '%s' already exists", role.Name)
		}
		if err := validateRoleName(role.Name); err != nil {
			return err
		}
		existingPolicies := make(map[string]bool)
		for _, policy := range role.Policies {
			if _, ok := existingPolicies[policy]; ok {
				return status.Errorf(codes.AlreadyExists, "policy '%s' already exists for role '%s'", policy, role.Name)
			}
			if err := validatePolicy(p.Name, role.Name, policy); err != nil {
				return err
			}
			existingPolicies[policy] = true
		}
		existingGroups := make(map[string]bool)
		for _, group := range role.Groups {
			if _, ok := existingGroups[group]; ok {
				return status.Errorf(codes.AlreadyExists, "group '%s' already exists for role '%s'", group, role.Name)
			}
			if err := validateGroupName(group); err != nil {
				return err
			}
			existingGroups[group] = true
		}
		roleNames[role.Name] = true
	}
	if err := validatePolicySyntax(p); err != nil {
		return err
	}
	return nil
}

// TODO: refactor to use rbacpolicy.ActionGet, rbacpolicy.ActionCreate, without import cycle
var validActions = map[string]bool{
	"get":    true,
	"create": true,
	"update": true,
	"delete": true,
	"sync":   true,
	"*":      true,
}

func isValidAction(action string) bool {
	return validActions[action]
}

func validatePolicy(proj string, role string, policy string) error {
	policyComponents := strings.Split(policy, ",")
	if len(policyComponents) != 6 || strings.Trim(policyComponents[0], " ") != "p" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': must be of the form: 'p, sub, res, act, obj, eft'", policy)
	}
	// subject
	subject := strings.Trim(policyComponents[1], " ")
	expectedSubject := fmt.Sprintf("proj:%s:%s", proj, role)
	if subject != expectedSubject {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': policy subject must be: '%s', not '%s'", policy, expectedSubject, subject)
	}
	// resource
	resource := strings.Trim(policyComponents[2], " ")
	if resource != "applications" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': project resource must be: 'applications', not '%s'", policy, resource)
	}
	// action
	action := strings.Trim(policyComponents[3], " ")
	if !isValidAction(action) {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': invalid action '%s'", policy, action)
	}
	// object
	object := strings.Trim(policyComponents[4], " ")
	objectRegexp, err := regexp.Compile(fmt.Sprintf(`^%s/[*\w-]+$`, proj))
	if err != nil || !objectRegexp.MatchString(object) {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': object must be of form '%s/*' or '%s/<APPNAME>', not '%s'", policy, proj, proj, object)
	}
	// effect
	effect := strings.Trim(policyComponents[5], " ")
	if effect != "allow" && effect != "deny" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': effect must be: 'allow' or 'deny'", policy)
	}
	return nil
}

var roleNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([-_a-zA-Z0-9]*[a-zA-Z0-9])?$`)

func validateRoleName(name string) error {
	if !roleNameRegexp.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid role name '%s'. Must consist of alphanumeric characters, '-' or '_', and must start and end with an alphanumeric character", name)
	}
	return nil
}

var invalidChars = regexp.MustCompile("[,\n\r\t]")

func validateGroupName(name string) error {
	if strings.TrimSpace(name) == "" {
		return status.Errorf(codes.InvalidArgument, "group '%s' is empty", name)
	}
	if invalidChars.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "group '%s' contains invalid characters", name)
	}
	return nil
}

// validatePolicySyntax verifies policy syntax is accepted by casbin
func validatePolicySyntax(p *v1alpha1.AppProject) error {
	err := rbac.ValidatePolicy(p.ProjectPoliciesString())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "policy syntax error: %s", err.Error())
	}
	return nil
}

// AddGroupToRole adds an OIDC group to a role
func AddGroupToRole(p *v1alpha1.AppProject, roleName, group string) (bool, error) {
	role, roleIndex, err := GetRoleByName(p, roleName)
	if err != nil {
		return false, err
	}
	for _, roleGroup := range role.Groups {
		if group == roleGroup {
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
	for i, roleGroup := range role.Groups {
		if group == roleGroup {
			role.Groups = append(role.Groups[0:i], role.Groups[i:]...)
			p.Spec.Roles[roleIndex] = *role
			return true, nil
		}
	}
	return false, nil
}

// NormalizePolicies normalizes the policies in the project
func NormalizePolicies(p *v1alpha1.AppProject) {
	for i, role := range p.Spec.Roles {
		var normalizedPolicies []string
		for _, policy := range role.Policies {
			normalizedPolicies = append(normalizedPolicies, normalizePolicy(policy))
		}
		p.Spec.Roles[i].Policies = normalizedPolicies
	}
}

func normalizePolicy(policy string) string {
	policyComponents := strings.Split(policy, ",")
	normalizedPolicy := ""
	for _, component := range policyComponents {
		if normalizedPolicy == "" {
			normalizedPolicy = component
		} else {
			normalizedPolicy = fmt.Sprintf("%s, %s", normalizedPolicy, strings.Trim(component, " "))
		}
	}
	return normalizedPolicy
}
