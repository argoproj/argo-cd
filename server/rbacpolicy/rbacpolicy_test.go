package rbacpolicy

import (
	"fmt"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	settings_util "github.com/argoproj/argo-cd/v3/util/settings"
)

func init() {
	settings_util.ConfigureGoClientFeatures()
}

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
						"p, proj:my-proj:my-role, logs, get, my-proj/*, allow",
						"p, proj:my-proj:my-role, exec, create, my-proj/*, allow",
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
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(newFakeProj())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableLog(true)
	_ = enf.SetBuiltinPolicy(`p, alice, applications, create, my-proj/*, allow` + "\n" + `p, alice, logs, get, my-proj/*, allow` + "\n" + `p, alice, exec, create, my-proj/*, allow`)
	_ = enf.SetUserPolicy(`p, bob, applications, create, my-proj/*, allow` + "\n" + `p, bob, logs, get, my-proj/*, allow` + "\n" + `p, bob, exec, create, my-proj/*, allow`)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)

	claims := jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "qwertyuiop", "federated_claims": map[string]any{"user_id": "bob"}}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "proj:my-proj:my-role", "iat": 1234}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"groups": []string{"my-org:my-team"}}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "cathy"}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	// AWS cognito returns its groups in  cognito:groups
	rbacEnf.SetScopes([]string{"cognito:groups"})
	claims = jwt.MapClaims{"cognito:groups": []string{"my-org:my-team"}}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
}

func TestEnforceActionActions(t *testing.T) {
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(newFakeProj())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableLog(true)
	_ = enf.SetBuiltinPolicy(fmt.Sprintf(`p, alice, applications, %s/*, my-proj/*, allow
p, bob, applications, %s/argoproj.io/Rollout/*, my-proj/*, allow
p, cam, applications, %s/argoproj.io/Rollout/resume, my-proj/*, allow
`, rbac.ActionAction, rbac.ActionAction, rbac.ActionAction))
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)

	// Alice has wild-card approval for all actions
	claims := jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/NewCrd/abort", "my-proj/my-app"))
	// Bob has wild-card approval for all actions under argoproj.io/Rollout
	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "bob"}
	assert.False(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/NewCrd/abort", "my-proj/my-app"))
	// Cam only has approval for actions/argoproj.io/Rollout:resume
	claims = jwt.MapClaims{"sub": "cam"}
	assert.True(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
	claims = jwt.MapClaims{"sub": "cam"}
	assert.False(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/Rollout/abort", "my-proj/my-app"))

	// Eve does not have approval for any actions
	claims = jwt.MapClaims{"sub": "eve"}
	assert.False(t, enf.Enforce(claims, "applications", rbac.ActionAction+"/argoproj.io/Rollout/resume", "my-proj/my-app"))
}

func TestInvalidatedCache(t *testing.T) {
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(newFakeProj())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableLog(true)
	_ = enf.SetBuiltinPolicy(`p, alice, applications, create, my-proj/*, allow` + "\n" + `p, alice, logs, get, my-proj/*, allow` + "\n" + `p, alice, exec, create, my-proj/*, allow`)
	_ = enf.SetUserPolicy(`p, bob, applications, create, my-proj/*, allow` + "\n" + `p, bob, logs, get, my-proj/*, allow` + "\n" + `p, bob, exec, create, my-proj/*, allow`)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)

	claims := jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	_ = enf.SetBuiltinPolicy(`p, alice, applications, create, my-proj2/*, allow` + "\n" + `p, alice, logs, get, my-proj2/*, allow` + "\n" + `p, alice, exec, create, my-proj2/*, allow`)
	_ = enf.SetUserPolicy(`p, bob, applications, create, my-proj2/*, allow` + "\n" + `p, bob, logs, get, my-proj2/*, allow` + "\n" + `p, bob, exec, create, my-proj2/*, allow`)
	claims = jwt.MapClaims{"sub": "alice"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj2/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj2/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj2/my-app"))

	claims = jwt.MapClaims{"sub": "bob"}
	assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj2/my-app"))
	assert.True(t, enf.Enforce(claims, "logs", "get", "my-proj2/my-app"))
	assert.True(t, enf.Enforce(claims, "exec", "create", "my-proj2/my-app"))

	claims = jwt.MapClaims{"sub": "alice"}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))

	claims = jwt.MapClaims{"sub": "bob"}
	assert.False(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "logs", "get", "my-proj/my-app"))
	assert.False(t, enf.Enforce(claims, "exec", "create", "my-proj/my-app"))
}

func TestGetScopes_DefaultScopes(t *testing.T) {
	rbacEnforcer := NewRBACPolicyEnforcer(nil, nil)

	scopes := rbacEnforcer.GetScopes()
	assert.Equal(t, scopes, rbac.DefaultScopes)
}

func TestGetScopes_CustomScopes(t *testing.T) {
	rbacEnforcer := NewRBACPolicyEnforcer(nil, nil)
	customScopes := []string{"custom"}
	rbacEnforcer.SetScopes(customScopes)

	scopes := rbacEnforcer.GetScopes()
	assert.Equal(t, scopes, customScopes)
}

func Test_getProjectFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		action   string
		arg      string
	}{
		{
			name:     "valid project/repo string",
			resource: "repositories",
			action:   "create",
			arg:      newFakeProj().Name + "/https://github.com/argoproj/argocd-example-apps",
		},
		{
			name:     "applicationsets with project/repo string",
			resource: "applicationsets",
			action:   "create",
			arg:      newFakeProj().Name + "/https://github.com/argoproj/argocd-example-apps",
		},
		{
			name:     "applicationsets with project/repo string",
			resource: "applicationsets",
			action:   "*",
			arg:      newFakeProj().Name + "/https://github.com/argoproj/argocd-example-apps",
		},
		{
			name:     "applicationsets with project/repo string",
			resource: "applicationsets",
			action:   "get",
			arg:      newFakeProj().Name + "/https://github.com/argoproj/argocd-example-apps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := newFakeProj()
			projLister := test.NewFakeProjLister(fp)
			rbacEnforcer := NewRBACPolicyEnforcer(nil, projLister)

			project := rbacEnforcer.getProjectFromRequest("", tt.resource, tt.action, tt.arg)
			require.Equal(t, fp.Name, project.Name)
		})
	}
}

func TestIsLocalAccount(t *testing.T) {
	assert.True(t, isLocalAccount(jwt.MapClaims{"iss": localAccountIssuer}))
	assert.False(t, isLocalAccount(jwt.MapClaims{"iss": "https://accounts.google.com"}))
	assert.False(t, isLocalAccount(jwt.MapClaims{"iss": "sso"}))
	assert.False(t, isLocalAccount(jwt.MapClaims{}))
}

func TestSetEnableLocalUserStrictMode(t *testing.T) {
	rbacEnf := NewRBACPolicyEnforcer(nil, nil)
	assert.False(t, rbacEnf.enableLocalUserStrictMode)
	rbacEnf.SetEnableLocalUserStrictMode(true)
	assert.True(t, rbacEnf.enableLocalUserStrictMode)
	rbacEnf.SetEnableLocalUserStrictMode(false)
	assert.False(t, rbacEnf.enableLocalUserStrictMode)
}

// TestEnforceLocalUserStrictMode verifies that, when strict mode is enabled, an RBAC policy bound
// to `sally@local` only grants permissions to the local `sally` account (iss=argocd) and not to an
// SSO user who happens to also be named `sally`. With strict mode disabled, both match the plain
// `sally` binding (the pre-existing, ambiguous behavior).
func TestEnforceLocalUserStrictMode(t *testing.T) {
	newEnforcer := func(t *testing.T) (*rbac.Enforcer, *RBACPolicyEnforcer) {
		t.Helper()
		kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
		projLister := test.NewFakeProjLister(newFakeProj())
		enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
		enf.EnableLog(true)
		rbacEnf := NewRBACPolicyEnforcer(enf, projLister)
		enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)
		return enf, rbacEnf
	}

	localClaims := jwt.MapClaims{"sub": "sally", "iss": localAccountIssuer}
	ssoClaims := jwt.MapClaims{"sub": "sally", "iss": "https://accounts.google.com"}

	t.Run("strict mode: only the local account matches an @local binding", func(t *testing.T) {
		enf, rbacEnf := newEnforcer(t)
		rbacEnf.SetEnableLocalUserStrictMode(true)
		_ = enf.SetBuiltinPolicy(`p, sally@local, applications, create, my-proj/*, allow`)

		assert.True(t, enf.Enforce(localClaims, "applications", "create", "my-proj/my-app"),
			"local sally should match the @local binding")
		assert.False(t, enf.Enforce(ssoClaims, "applications", "create", "my-proj/my-app"),
			"SSO sally must not inherit the local user's role")
	})

	t.Run("strict mode: a plain binding no longer matches the local account", func(t *testing.T) {
		enf, rbacEnf := newEnforcer(t)
		rbacEnf.SetEnableLocalUserStrictMode(true)
		_ = enf.SetBuiltinPolicy(`p, sally, applications, create, my-proj/*, allow`)

		assert.False(t, enf.Enforce(localClaims, "applications", "create", "my-proj/my-app"),
			"local sally is enforced as sally@local and should not match the plain binding")
		assert.True(t, enf.Enforce(ssoClaims, "applications", "create", "my-proj/my-app"),
			"SSO sally is not suffixed and still matches the plain binding")
	})

	t.Run("strict mode disabled: both local and SSO match a plain binding", func(t *testing.T) {
		enf, _ := newEnforcer(t)
		_ = enf.SetBuiltinPolicy(`p, sally, applications, create, my-proj/*, allow`)

		assert.True(t, enf.Enforce(localClaims, "applications", "create", "my-proj/my-app"))
		assert.True(t, enf.Enforce(ssoClaims, "applications", "create", "my-proj/my-app"))
	})

	t.Run("strict mode: project tokens are left untouched", func(t *testing.T) {
		enf, rbacEnf := newEnforcer(t)
		rbacEnf.SetEnableLocalUserStrictMode(true)
		claims := jwt.MapClaims{"sub": "proj:my-proj:my-role", "iat": 1234}
		assert.True(t, enf.Enforce(claims, "applications", "create", "my-proj/my-app"),
			"project role tokens must not be suffixed with @local")
	})
}
