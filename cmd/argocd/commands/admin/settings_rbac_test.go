package admin

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/util/assets"
)

func Test_isValidRBACAction(t *testing.T) {
	for k := range validRBACActions {
		t.Run(k, func(t *testing.T) {
			ok := isValidRBACAction(k)
			assert.True(t, ok)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		ok := isValidRBACAction("invalid")
		assert.False(t, ok)
	})
}

func Test_isValidRBACResource(t *testing.T) {
	for k := range validRBACResources {
		t.Run(k, func(t *testing.T) {
			ok := isValidRBACResource(k)
			assert.True(t, ok)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		ok := isValidRBACResource("invalid")
		assert.False(t, ok)
	})
}

func Test_PolicyFromCSV(t *testing.T) {
	ctx := context.Background()

	uPol, dRole, matchMode := getPolicy(ctx, "testdata/rbac/policy.csv", nil, "")
	require.NotEmpty(t, uPol)
	require.Empty(t, dRole)
	require.Empty(t, matchMode)
}

func Test_PolicyFromYAML(t *testing.T) {
	ctx := context.Background()

	uPol, dRole, matchMode := getPolicy(ctx, "testdata/rbac/argocd-rbac-cm.yaml", nil, "")
	require.NotEmpty(t, uPol)
	require.Equal(t, "role:unknown", dRole)
	require.Empty(t, matchMode)
}

func Test_PolicyFromK8s(t *testing.T) {
	data, err := os.ReadFile("testdata/rbac/policy.csv")
	ctx := context.Background()

	require.NoError(t, err)
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-rbac-cm",
			Namespace: "argocd",
		},
		Data: map[string]string{
			"policy.csv":     string(data),
			"policy.default": "role:unknown",
		},
	})
	uPol, dRole, matchMode := getPolicy(ctx, "", kubeclientset, "argocd")
	require.NotEmpty(t, uPol)
	require.Equal(t, "role:unknown", dRole)
	require.Equal(t, "", matchMode)

	t.Run("get applications", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "applications", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
	t.Run("get clusters", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "clusters", "*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
	t.Run("get certificates", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.False(t, ok)
	})
	t.Run("get certificates by default role", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", assets.BuiltinPolicyCSV, uPol, "role:readonly", "glob", true)
		require.True(t, ok)
	})
	t.Run("get certificates by default role without builtin policy", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", "*", "", uPol, "role:readonly", "glob", true)
		require.False(t, ok)
	})
	t.Run("use regex match mode instead of glob", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", ".*", assets.BuiltinPolicyCSV, uPol, "role:readonly", "regex", true)
		require.False(t, ok)
	})
}

func Test_PolicyFromK8sUsingRegex(t *testing.T) {
	ctx := context.Background()

	policy := `
p, role:user, clusters, get, .+, allow
p, role:user, clusters, get, https://kubernetes.*, deny
p, role:user, applications, get, .*, allow
p, role:user, applications, create, .*/.*, allow`

	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-rbac-cm",
			Namespace: "argocd",
		},
		Data: map[string]string{
			"policy.csv":       policy,
			"policy.default":   "role:unknown",
			"policy.matchMode": "regex",
		},
	})
	uPol, dRole, matchMode := getPolicy(ctx, "", kubeclientset, "argocd")
	require.NotEmpty(t, uPol)
	require.Equal(t, "role:unknown", dRole)
	require.Equal(t, "regex", matchMode)

	builtInPolicy := `
p, role:readonly, certificates, get, .*, allow
p, role:, certificates, get, .*, allow`

	t.Run("get applications", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "applications", ".*/.*", builtInPolicy, uPol, dRole, "regex", true)
		require.True(t, ok)
	})
	t.Run("get clusters", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "clusters", ".*", builtInPolicy, uPol, dRole, "regex", true)
		require.True(t, ok)
	})
	t.Run("get certificates", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", ".*", builtInPolicy, uPol, dRole, "regex", true)
		require.False(t, ok)
	})
	t.Run("get certificates by default role", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", ".*", builtInPolicy, uPol, "role:readonly", "regex", true)
		require.True(t, ok)
	})
	t.Run("get certificates by default role without builtin policy", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", ".*", "", uPol, "role:readonly", "regex", true)
		require.False(t, ok)
	})
	t.Run("use glob match mode instead of regex", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "certificates", ".+", builtInPolicy, uPol, dRole, "glob", true)
		require.False(t, ok)
	})
}
