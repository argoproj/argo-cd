package rbacpolicy

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestUserHasAnyPermission(t *testing.T) {
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister()
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)

	tests := []struct {
		name     string
		policy   string
		username string
		groups   []string
		expected bool
	}{
		{
			name:     "user with direct allow permission",
			policy:   "p, alice, applications, get, *, allow",
			username: "alice",
			expected: true,
		},
		{
			name:     "user with no permissions",
			policy:   "p, bob, applications, get, *, allow",
			username: "alice",
			expected: false,
		},
		{
			name:     "user with only deny",
			policy:   "p, alice, applications, get, *, deny",
			username: "alice",
			expected: false,
		},
		{
			name:     "permission via group membership",
			policy:   "p, my-group, applications, get, *, allow",
			username: "alice",
			groups:   []string{"my-group"},
			expected: true,
		},
		{
			name:     "no match on wrong group",
			policy:   "p, other-group, applications, get, *, allow",
			username: "alice",
			groups:   []string{"my-group"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, enf.SetUserPolicy(tt.policy))
			enf.SetDefaultRole("")
			assert.Equal(t, tt.expected, rbacEnf.UserHasAnyPermission(tt.username, tt.groups))
		})
	}
}

func TestUserHasAnyPermission_DefaultRole(t *testing.T) {
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister()
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)

	require.NoError(t, enf.SetBuiltinPolicy("p, role:readonly, applications, get, *, allow"))
	enf.SetDefaultRole("role:readonly")

	// user with no direct policy still has permission via the default role
	assert.True(t, rbacEnf.UserHasAnyPermission("alice", nil))
}

func TestUserHasAnyPermission_NilProjLister(t *testing.T) {
	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	rbacEnf := NewRBACPolicyEnforcer(enf, nil)

	assert.False(t, rbacEnf.UserHasAnyPermission("alice", nil),
		"nil projLister should not panic and should return false")
}

func TestUserHasAnyPermission_ProjectScope(t *testing.T) {
	proj := &argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-project",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: argoappv1.AppProjectSpec{
			Roles: []argoappv1.ProjectRole{
				{
					Name:     "developer",
					Policies: []string{"p, proj:my-project:developer, applications, sync, my-project/*, allow"},
					Groups:   []string{"dev-team"},
				},
			},
		},
	}

	kubeclientset := fake.NewClientset(test.NewFakeConfigMap())
	projLister := test.NewFakeProjLister(proj)
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	rbacEnf := NewRBACPolicyEnforcer(enf, projLister)

	assert.True(t, rbacEnf.UserHasAnyPermission("alice", []string{"dev-team"}),
		"user bound only to a project role should pass the permission gate")

	assert.False(t, rbacEnf.UserHasAnyPermission("bob", nil),
		"user with no global or project permissions should be blocked")
}

func newEnforcerWithInMemoryCache(t *testing.T) (*rbac.Enforcer, *Enforcer) {
	t.Helper()
	enf := rbac.NewEnforcer(fake.NewClientset(test.NewFakeConfigMap()), test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	rbacEnf := NewRBACPolicyEnforcer(enf, test.NewFakeProjLister())
	rbacEnf.SetPermCheckCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Hour)))
	return enf, rbacEnf
}

// TestUserHasAnyPermission_CacheHit verifies that after the first call the result
// is served from cache: a subsequent policy change is NOT reflected until the
// cache is flushed.
func TestUserHasAnyPermission_CacheHit(t *testing.T) {
	enf, rbacEnf := newEnforcerWithInMemoryCache(t)
	require.NoError(t, enf.SetUserPolicy("p, alice, applications, get, *, allow"))

	// First call — populates the cache with "allowed".
	assert.True(t, rbacEnf.UserHasAnyPermission("alice", nil))

	// Remove alice's permission without flushing the cache.
	require.NoError(t, enf.SetUserPolicy(""))

	// Second call — still returns the cached "allowed" result.
	assert.True(t, rbacEnf.UserHasAnyPermission("alice", nil))
}

// TestUserHasAnyPermission_FlushInvalidatesCache verifies that FlushPermCheckCache
// causes the next call to re-run the live Casbin lookup and pick up the new policy.
func TestUserHasAnyPermission_FlushInvalidatesCache(t *testing.T) {
	enf, rbacEnf := newEnforcerWithInMemoryCache(t)
	require.NoError(t, enf.SetUserPolicy("p, alice, applications, get, *, allow"))

	// Populate cache with "allowed".
	assert.True(t, rbacEnf.UserHasAnyPermission("alice", nil))

	// Remove alice's permission and invalidate the cache.
	require.NoError(t, enf.SetUserPolicy(""))
	rbacEnf.FlushPermCheckCache()

	// Next call re-runs Casbin and reflects the revocation.
	assert.False(t, rbacEnf.UserHasAnyPermission("alice", nil))
}

// TestUserHasAnyPermission_FlushAllowsGrant verifies the symmetric case: a user
// that was denied gets access after a grant + flush.
func TestUserHasAnyPermission_FlushAllowsGrant(t *testing.T) {
	enf, rbacEnf := newEnforcerWithInMemoryCache(t)
	require.NoError(t, enf.SetUserPolicy(""))

	// Populate cache with "denied".
	assert.False(t, rbacEnf.UserHasAnyPermission("alice", nil))

	// Grant a permission and invalidate the cache.
	require.NoError(t, enf.SetUserPolicy("p, alice, applications, get, *, allow"))
	rbacEnf.FlushPermCheckCache()

	// Next call picks up the new grant.
	assert.True(t, rbacEnf.UserHasAnyPermission("alice", nil))
}

// TestPermCheckCacheKey verifies that the cache key is stable and group-order independent.
func TestPermCheckCacheKey(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		groups      []string
		sameAsKey   string
		sameGroups  []string
		expectEqual bool
	}{
		{
			name:        "same groups different order produce equal key",
			username:    "alice",
			groups:      []string{"b", "a", "c"},
			sameAsKey:   "alice",
			sameGroups:  []string{"a", "b", "c"},
			expectEqual: true,
		},
		{
			name:        "different users produce different keys",
			username:    "alice",
			groups:      nil,
			sameAsKey:   "bob",
			sameGroups:  nil,
			expectEqual: false,
		},
		{
			name:        "different groups produce different keys",
			username:    "alice",
			groups:      []string{"group-a"},
			sameAsKey:   "alice",
			sameGroups:  []string{"group-b"},
			expectEqual: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k1 := permCheckCacheKey(tt.username, tt.groups)
			k2 := permCheckCacheKey(tt.sameAsKey, tt.sameGroups)
			if tt.expectEqual {
				assert.Equal(t, k1, k2)
			} else {
				assert.NotEqual(t, k1, k2)
			}
		})
	}
}

func TestGetScopes_DefaultScopes(t *testing.T) {
	t.Parallel()
	rbacEnforcer := NewRBACPolicyEnforcer(nil, nil)

	scopes := rbacEnforcer.GetScopes()
	assert.Equal(t, scopes, rbac.DefaultScopes)
}

func TestGetScopes_CustomScopes(t *testing.T) {
	t.Parallel()
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
