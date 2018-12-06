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
		"my, role",
		"my,role",
		"my\nrole",
		"my\rrole",
	}
	for _, badName := range badRoleNames {
		p.Spec.Roles[0].Name = badName
		err = ValidateProject(p)
		assert.Error(t, err)
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
}

// TestValidatePolicySyntax we detect when policy format is not accepted by casbin
func TestValidatePolicySyntax(t *testing.T) {
	p := newTestProject()
	err := ValidateProject(p)
	assert.NoError(t, err)
	p.Spec.Roles[0].Policies = []string{"p,should,have,spaces,my-proj/*,allow"}
	err = ValidateProject(p)
	assert.Contains(t, err.Error(), "syntax")
}
