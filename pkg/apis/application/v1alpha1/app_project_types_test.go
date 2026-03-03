package v1alpha1

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppProject_NormalizePolicies(t *testing.T) {
	namespace := "argocd"
	t.Run("EmptyPolicies", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name:     "test-role",
						Policies: []string{},
					},
				},
			},
		}

		proj.NormalizePolicies()

		assert.Empty(t, proj.Spec.Roles[0].Policies)
	})

	t.Run("RemoveDuplicatePolicies", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "test-role",
						Policies: []string{
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, sync, test-project/*, allow",
						},
					},
				},
			},
		}

		proj.NormalizePolicies()

		assert.Len(t, proj.Spec.Roles[0].Policies, 2)
	})

	t.Run("MultipleRoles", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "role1",
						Policies: []string{
							"p, proj:test-project:role1, applications, get, test-project/*, allow",
							"p, proj:test-project:role1, applications, get, test-project/*, allow",
						},
					},
					{
						Name: "role2",
						Policies: []string{
							"p, proj:test-project:role2, applications, sync, test-project/*, allow",
						},
					},
				},
			},
		}

		proj.NormalizePolicies()

		assert.Len(t, proj.Spec.Roles[0].Policies, 1)
		assert.Len(t, proj.Spec.Roles[1].Policies, 1)
	})
}

func TestAppProject_ProjectPoliciesString(t *testing.T) {
	namespace := "argocd"
	t.Run("SingleRoleWithoutGroups", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "test-role",
						Policies: []string{
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, get, test-project/test-ns/*, allow",
						},
					},
				},
			},
		}

		result := proj.ProjectPoliciesString()

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 3)
		assert.Contains(t, result, "p, proj:test-project:test-role, projects, get, test-project, allow")
		assert.Contains(t, result, fmt.Sprintf("p, proj:test-project:test-role, applications, get, test-project/%s/*, allow", namespace))
		assert.NotContains(t, result, "p, proj:test-project:test-role, applications, get, test-project/*, allow")
		assert.Contains(t, result, "p, proj:test-project:test-role, applications, get, test-project/test-ns/*, allow")
	})

	t.Run("SingleRoleWithGroups", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "test-role",
						Policies: []string{
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, get, test-project/test-ns/*, allow",
						},
						Groups: []string{"admin-group", "viewer-group"},
					},
				},
			},
		}

		result := proj.ProjectPoliciesString()

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 5)
		assert.Contains(t, result, "p, proj:test-project:test-role, projects, get, test-project, allow")
		assert.Contains(t, result, fmt.Sprintf("p, proj:test-project:test-role, applications, get, test-project/%s/*, allow", namespace))
		assert.NotContains(t, result, "p, proj:test-project:test-role, applications, get, test-project/*, allow")
		assert.Contains(t, result, "p, proj:test-project:test-role, applications, get, test-project/test-ns/*, allow")
		assert.Contains(t, result, "g, admin-group, proj:test-project:test-role")
		assert.Contains(t, result, "g, viewer-group, proj:test-project:test-role")
	})

	t.Run("MultipleRoles", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "admin",
						Policies: []string{
							"p, proj:test-project:admin, applications, *, test-project/*, allow",
						},
						Groups: []string{"admin-group"},
					},
					{
						Name: "viewer",
						Policies: []string{
							"p, proj:test-project:viewer, applications, get, test-project/*, allow",
						},
						Groups: []string{"viewer-group"},
					},
				},
			},
		}

		result := proj.ProjectPoliciesString()

		assert.Contains(t, result, "p, proj:test-project:admin, projects, get, test-project, allow")
		assert.Contains(t, result, "p, proj:test-project:viewer, projects, get, test-project, allow")
		assert.Contains(t, result, "g, admin-group, proj:test-project:admin")
		assert.Contains(t, result, "g, viewer-group, proj:test-project:viewer")
	})

	t.Run("DeduplicatePolicies", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name: "test-role",
						Policies: []string{
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, get, test-project/*, allow",
							"p, proj:test-project:test-role, applications, sync, test-project/*, allow",
						},
					},
				},
			},
		}

		result := proj.ProjectPoliciesString()

		lines := strings.Split(result, "\n")
		// Should have: 1 project policy + 2 unique application policies = 3 lines
		assert.Len(t, lines, 3)
	})

	t.Run("EmptyRole", func(t *testing.T) {
		proj := &AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: namespace},
			Spec: AppProjectSpec{
				Roles: []ProjectRole{
					{
						Name:     "test-role",
						Policies: []string{},
						Groups:   []string{},
					},
				},
			},
		}

		result := proj.ProjectPoliciesString()

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 1)
		assert.Contains(t, result, "p, proj:test-project:test-role, projects, get, test-project, allow")
	})
}
