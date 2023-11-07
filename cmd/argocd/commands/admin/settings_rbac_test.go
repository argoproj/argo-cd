package admin

import (
	"context"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type FakeClientConfig struct {
	clientConfig clientcmd.ClientConfig
}

func NewFakeClientConfig(clientConfig clientcmd.ClientConfig) *FakeClientConfig {
	return &FakeClientConfig{clientConfig: clientConfig}
}

func (f *FakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	config, err := f.clientConfig.RawConfig()
	return config, err
}

func (f *FakeClientConfig) ClientConfig() (*restclient.Config, error) {
	return f.clientConfig.ClientConfig()
}

func (f *FakeClientConfig) Namespace() (string, bool, error) {
	return f.clientConfig.Namespace()
}

func (f *FakeClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

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

func Test_isValidRBACAction_ActionAction(t *testing.T) {
	ok := isValidRBACAction("action/apps/Deployment/restart")
	assert.True(t, ok)
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
	t.Run("get logs", func(t *testing.T) {
		ok := checkPolicy("role:test", "get", "logs", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
	t.Run("create exec", func(t *testing.T) {
		ok := checkPolicy("role:test", "create", "exec", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
	t.Run("create applicationsets", func(t *testing.T) {
		ok := checkPolicy("role:user", "create", "applicationsets", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
	t.Run("delete applicationsets", func(t *testing.T) {
		ok := checkPolicy("role:user", "delete", "applicationsets", "*/*", assets.BuiltinPolicyCSV, uPol, dRole, "", true)
		require.True(t, ok)
	})
}

func Test_PolicyFromK8sUsingRegex(t *testing.T) {
	ctx := context.Background()

	policy := `
p, role:user, clusters, get, .+, allow
p, role:user, clusters, get, https://kubernetes.*, deny
p, role:user, applications, get, .*, allow
p, role:user, applications, create, .*/.*, allow
p, role:user, applicationsets, create, .*/.*, allow
p, role:user, applicationsets, delete, .*/.*, allow
p, role:user, logs, get, .*/.*, allow
p, role:user, exec, create, .*/.*, allow
`

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
	t.Run("get logs via glob match mode", func(t *testing.T) {
		ok := checkPolicy("role:user", "get", "logs", ".*/.*", builtInPolicy, uPol, dRole, "glob", true)
		require.True(t, ok)
	})
	t.Run("create exec", func(t *testing.T) {
		ok := checkPolicy("role:user", "create", "exec", ".*/.*", builtInPolicy, uPol, dRole, "regex", true)
		require.True(t, ok)
	})
	t.Run("create applicationsets", func(t *testing.T) {
		ok := checkPolicy("role:user", "create", "applicationsets", ".*/.*", builtInPolicy, uPol, dRole, "regex", true)
		require.True(t, ok)
	})
	t.Run("delete applicationsets", func(t *testing.T) {
		ok := checkPolicy("role:user", "delete", "applicationsets", ".*/.*", builtInPolicy, uPol, dRole, "regex", true)
		require.True(t, ok)
	})
}

func TestNewRBACCanCommand(t *testing.T) {
	command := NewRBACCanCommand()

	require.NotNil(t, command)
	assert.Equal(t, "can", command.Name())
	assert.Equal(t, "Check RBAC permissions for a role or subject", command.Short)
}

func TestNewRBACValidateCommand(t *testing.T) {
	command := NewRBACValidateCommand()

	require.NotNil(t, command)
	assert.Equal(t, "validate", command.Name())
	assert.Equal(t, "Validate RBAC policy", command.Short)
}
