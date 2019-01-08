package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func newTestProject() *argoappv1.AppProject {
	p := argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-proj",
		},
		Spec: argoappv1.AppProjectSpec{
			Roles: []argoappv1.ProjectRole{
				{
					Name: "my-role",
				},
			},
		},
	}
	return &p
}

// TestValidateRoleName tests for an invalid role name
func TestValidateRoleName(t *testing.T) {
	p := newTestProject()
	err := ValidateProject(p)
	assert.NoError(t, err)
	badRoleNames := []string{
		"",
		" ",
		"my role",
		"my, role",
		"my,role",
		"my\nrole",
		"my\rrole",
		"my:role",
		"my-role-",
		"-my-role",
	}
	for _, badName := range badRoleNames {
		p.Spec.Roles[0].Name = badName
		err = ValidateProject(p)
		assert.Error(t, err)
	}
	goodRoleNames := []string{
		"MY-ROLE",
		"1MY-ROLE1",
	}
	for _, goodName := range goodRoleNames {
		p.Spec.Roles[0].Name = goodName
		err = ValidateProject(p)
		assert.NoError(t, err)
	}
}

// TestValidateGroupName tests for an invalid group name
func TestValidateGroupName(t *testing.T) {
	p := newTestProject()
	err := ValidateProject(p)
	assert.NoError(t, err)
	p.Spec.Roles[0].Groups = []string{"mygroup"}
	err = ValidateProject(p)
	assert.NoError(t, err)
	badGroupNames := []string{
		"",
		" ",
		"my, group",
		"my,group",
		"my\ngroup",
		"my\rgroup",
	}
	for _, badName := range badGroupNames {
		p.Spec.Roles[0].Groups = []string{badName}
		err = ValidateProject(p)
		assert.Error(t, err)
	}
	goodGroupNames := []string{
		"my:group",
	}
	for _, goodName := range goodGroupNames {
		p.Spec.Roles[0].Groups = []string{goodName}
		err = ValidateProject(p)
		assert.NoError(t, err)
	}
}

// TestInvalidPolicyRules checks various errors in policy rules
func TestInvalidPolicyRules(t *testing.T) {
	p := newTestProject()
	err := ValidateProject(p)
	assert.NoError(t, err)
	type badPolicy struct {
		policy string
		errmsg string
	}
	badPolicies := []badPolicy{
		// should have spaces
		{"p,proj:my-proj:my-role,applications,get,my-proj/*,allow", "syntax"},
		// incorrect form
		{"g, proj:my-proj:my-role, applications, get, my-proj/*, allow", "must be of the form: 'p, sub, res, act, obj, eft'"},
		{"p, not, enough, parts", "must be of the form: 'p, sub, res, act, obj, eft'"},
		{"p, has, too, many, parts, to, split", "must be of the form: 'p, sub, res, act, obj, eft'"},
		// invalid subject
		{"p, , applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:my-proj, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:my-proj:, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, ::, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		{"p, proj:different-my-proj:my-role, applications, get, my-proj/*, allow", "policy subject must be: 'proj:my-proj:my-role'"},
		// invalid resource
		{"p, proj:my-proj:my-role, , get, my-proj/*, allow", "resource must be: 'applications'"},
		{"p, proj:my-proj:my-role, applicationz, get, my-proj/*, allow", "resource must be: 'applications'"},
		{"p, proj:my-proj:my-role, projects, get, my-proj, allow", "resource must be: 'applications'"},
		// invalid action
		{"p, proj:my-proj:my-role, applications, , my-proj/*, allow", "invalid action"},
		{"p, proj:my-proj:my-role, applications, foo, my-proj/*, allow", "invalid action"},
		// invalid object
		{"p, proj:my-proj:my-role, applications, get, my-proj/, allow", "object must be of form"},
		{"p, proj:my-proj:my-role, applications, get, /, allow", "object must be of form"},
		{"p, proj:my-proj:my-role, applications, get, different-my-proj/*, allow", "object must be of form"},
		// invalid effect
		{"p, proj:my-proj:my-role, applications, get, my-proj/*, ", "effect must be: 'allow' or 'deny'"},
		{"p, proj:my-proj:my-role, applications, get, my-proj/*, foo", "effect must be: 'allow' or 'deny'"},
	}
	for _, bad := range badPolicies {
		p.Spec.Roles[0].Policies = []string{bad.policy}
		err = ValidateProject(p)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), bad.errmsg)
		}
	}
}

// TestValidPolicyRules checks valid policy rules
func TestValidPolicyRules(t *testing.T) {
	p := newTestProject()
	err := ValidateProject(p)
	assert.NoError(t, err)
	goodPolicies := []string{
		"p, proj:my-proj:my-role, applications, get, my-proj/*, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*, deny",
		"p, proj:my-proj:my-role, applications, get, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*-foo, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/foo-*, allow",
		"p, proj:my-proj:my-role, applications, get, my-proj/*-*, allow",
		"p, proj:my-proj:my-role, applications, *, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, create, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, update, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, sync, my-proj/foo, allow",
		"p, proj:my-proj:my-role, applications, delete, my-proj/foo, allow",
	}
	for _, good := range goodPolicies {
		p.Spec.Roles[0].Policies = []string{good}
		err = ValidateProject(p)
		assert.NoError(t, err)
	}
}
