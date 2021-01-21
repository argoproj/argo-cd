package rbacpolicy

import (
	"fmt"
	"testing"

	jwt "github.com/dgrijalva/jwt-go/v4"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/rbac"
)

func newFakeProj() *argoappv1.AppProject {
	jwtTokenByRole := make(map[string]argoappv1.JWTTokens)
	jwtTokenByRole["my-role"] = argoappv1.JWTTokens{Items: []argoappv1.JWTToken{{IssuedAt: 1234}}}

	return &argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-proj",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: argoappv1.AppProjectSpec{
			Roles: []argoappv1.ProjectRole{
				{
					Name: "my-role",
					Policies: []string{
						"p, proj:my-proj:my-role, applications, create, my-proj/*, allow",
					},
					Groups: []string{
						"my-org:my-team",
					},
					JWTTokens: []argoappv1.JWTToken{
						{
							IssuedAt: 1234,
						},
					},
				},
			},
		},
		Status: argoappv1.AppProjectStatus{JWTTokensByRole: jwtTokenByRole},
	}
}

func TestEnforceAllPolicies(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(newFakeProj())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableLog(true)
	_ = enf.SetBuiltinPolicy(`p, alice, applications, create, my-proj/*, allow`)
	_ = enf.SetUserPolicy(`p, bob, applications, create, my-proj/*, allow`)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)

	claims := jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "proj:my-proj:my-role", "iat": 1234}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"groups": []string{"my-org:my-team"}}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "cathy"}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "proj:my-proj:my-role"}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "proj:my-proj:other-role", "iat": 1234}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	claims = jwt.MapClaims{"groups": []string{"my-org:other-group"}}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))

	// AWS cognito returns its groups in  cognito:groups
	rbacEnf.SetScopes([]string{"cognito:groups"})
	claims = jwt.MapClaims{"cognito:groups": []string{"my-org:my-team"}}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
}

func TestEnforceActionActions(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(newFakeProj())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableLog(true)
	_ = enf.SetBuiltinPolicy(fmt.Sprintf(`p, alice, applications, %s/*, my-proj/*, allow
p, bob, applications, %s/argoproj.io/Rollout/*, my-proj/*, allow
p, cam, applications, %s/argoproj.io/Rollout/resume, my-proj/*, allow
`, ActionAction, ActionAction, ActionAction))
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)

	// Alice has wild-card approval for all actions
	claims := jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/NewCrd/abort", "my-proj/my-app"))
	// Bob has wild-card approval for all actions under argoproj.io/Rollout
	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "bob"}
	assert.False(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/NewCrd/abort", "my-proj/my-app"))
	// Cam only has approval for actions/argoproj.io/Rollout:resume
	claims = jwt.MapClaims{"sub": "cam"}
	assert.True(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "cam"}
	assert.False(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/Rollout/abort", "my-proj/my-app"))

	// Eve does not have approval for any actions
	claims = jwt.MapClaims{"sub": "eve"}
	assert.False(t, enf.Enforce(claims, "applications", ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
}

func TestGetScopes_DefaultScopes(t *testing.T) {
	rbacEnforcer := NewRBACPolicyEnforcer(nil, nil)

	scopes := rbacEnforcer.GetScopes()
	assert.Equal(t, scopes, defaultScopes)
}

func TestGetScopes_CustomScopes(t *testing.T) {
	rbacEnforcer := NewRBACPolicyEnforcer(nil, nil)
	customScopes := []string{"custom"}
	rbacEnforcer.SetScopes(customScopes)

	scopes := rbacEnforcer.GetScopes()
	assert.Equal(t, scopes, customScopes)
}
