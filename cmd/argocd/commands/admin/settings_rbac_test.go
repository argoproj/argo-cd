package admin

import (
	"context"
	"os"
	"testing"

	fakeappclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/assets"
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

func Test_validateRBACResourceAction(t *testing.T) {
	type args struct {
		resource string
		action   string
	}
	tests := []struct {
		name  string
		args  args
		valid bool
	}{
		{
			name: "Test valid resource and action",
			args: args{
				resource: rbacpolicy.ResourceApplications,
				action:   rbacpolicy.ActionCreate,
			},
			valid: true,
		},
		{
			name: "Test invalid resource",
			args: args{
				resource: "invalid",
			},
			valid: false,
		},
		{
			name: "Test invalid action",
			args: args{
				resource: rbacpolicy.ResourceApplications,
				action:   "invalid",
			},
			valid: false,
		},
		{
			name: "Test invalid action for resource",
			args: args{
				resource: rbacpolicy.ResourceLogs,
				action:   rbacpolicy.ActionCreate,
			},
			valid: false,
		},
		{
			name: "Test valid action with path",
			args: args{
				resource: rbacpolicy.ResourceApplications,
				action:   rbacpolicy.ActionAction + "/apps/Deployment/restart",
			},
			valid: true,
		},
		{
			name: "Test invalid action with path",
			args: args{
				resource: rbacpolicy.ResourceApplications,
				action:   rbacpolicy.ActionGet + "/apps/Deployment/restart",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRBACResourceAction(tt.args.resource, tt.args.action)
			if tt.valid {
				assert.NoError(t, result)
			} else {
				assert.Error(t, result)
			}
		})
	}
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
	require.True(t, checkPolicy("", "my-org:team-qa", rbacpolicy.ActionUpdate, rbacpolicy.ResourceProjects, "foo", "", uPol, "", dRole, matchMode, true, nil))
}

func Test_PolicyFromK8s(t *testing.T) {
	data, err := os.ReadFile("testdata/rbac/policy.csv")
	ctx := context.Background()

	require.NoError(t, err)
	kubeclientset := fake.NewClientset(&v1.ConfigMap{
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
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.True(t, ok)
	})
	t.Run("get clusters", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceClusters, "*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.True(t, ok)
	})
	t.Run("get certificates", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, "*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.False(t, ok)
	})
	t.Run("get certificates by default role", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, "*", assets.BuiltinPolicyCSV, uPol, "", "role:readonly", "glob", true, nil)
		require.True(t, ok)
	})
	t.Run("get certificates by default role without builtin policy", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, "*", "", uPol, "", "role:readonly", "glob", true, nil)
		require.False(t, ok)
	})
	t.Run("use regex match mode instead of glob", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, ".*", assets.BuiltinPolicyCSV, uPol, "", "role:readonly", "regex", true, nil)
		require.False(t, ok)
	})
	t.Run("get logs", func(t *testing.T) {
		ok := checkPolicy("", "role:test", rbacpolicy.ActionGet, rbacpolicy.ResourceLogs, "*/*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.True(t, ok)
	})
	t.Run("create exec", func(t *testing.T) {
		ok := checkPolicy("", "role:test", rbacpolicy.ActionCreate, rbacpolicy.ResourceExec, "*/*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.True(t, ok)
	})
	t.Run("create applicationsets", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
		require.True(t, ok)
	})
	t.Run("delete applicationsets", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, "", dRole, "", true, nil)
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

	kubeclientset := fake.NewClientset(&v1.ConfigMap{
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
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplications, ".*/.*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.True(t, ok)
	})
	t.Run("get clusters", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceClusters, ".*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.True(t, ok)
	})
	t.Run("get certificates", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, ".*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.False(t, ok)
	})
	t.Run("get certificates by default role", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, ".*", builtInPolicy, uPol, "", "role:readonly", "regex", true, nil)
		require.True(t, ok)
	})
	t.Run("get certificates by default role without builtin policy", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, ".*", "", uPol, "", "role:readonly", "regex", true, nil)
		require.False(t, ok)
	})
	t.Run("use glob match mode instead of regex", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceCertificates, ".+", builtInPolicy, uPol, "", dRole, "glob", true, nil)
		require.False(t, ok)
	})
	t.Run("get logs via glob match mode", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceLogs, ".*/.*", builtInPolicy, uPol, "", dRole, "glob", true, nil)
		require.True(t, ok)
	})
	t.Run("create exec", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceExec, ".*/.*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.True(t, ok)
	})
	t.Run("create applicationsets", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplicationSets, ".*/.*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.True(t, ok)
	})
	t.Run("delete applicationsets", func(t *testing.T) {
		ok := checkPolicy("", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplicationSets, ".*/.*", builtInPolicy, uPol, "", dRole, "regex", true, nil)
		require.True(t, ok)
	})
}

func Test_PolicyFromAppProjects(t *testing.T) {
	ctx := context.Background()

	policy := `
g, role:user, proj:foo:test
g, role:user, proj:bar:test
`

	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-rbac-cm",
			Namespace: "argocd",
		},
		Data: map[string]string{
			"policy.csv":       policy,
			"policy.default":   "",
			"policy.matchMode": "glob",
		},
	})

	appclients := fakeappclientset.NewSimpleClientset(newProj("foo", "test"))
	projIf := appclients.ArgoprojV1alpha1().AppProjects(namespace)
	uPol, dRole, matchMode := getPolicy(ctx, "", kubeclientset, "argocd")

	require.Equal(t, policy, uPol)
	require.Empty(t, dRole)
	require.Equal(t, "glob", matchMode)

	t.Run("get applications", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplications, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionGet, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("*", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")

		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplications, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("create applications", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplications, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionCreate, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplications, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("update applications", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplications, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionUpdate, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionUpdate, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionUpdate, rbacpolicy.ResourceApplications, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("delete applications", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplications, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionDelete, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplications, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("sync applications", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplications, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionSync, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionSync, rbacpolicy.ResourceApplications, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionSync, rbacpolicy.ResourceApplications, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("get applicationsets", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplicationSets, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionGet, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceApplicationSets, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("create applicationsets", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplicationSets, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionCreate, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceApplicationSets, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("update applicationsets", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplicationSets, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionUpdate, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionUpdate, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionUpdate, rbacpolicy.ResourceApplicationSets, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("delete applicationsets", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceApplicationSets, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionDelete, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplicationSets, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionDelete, rbacpolicy.ResourceApplicationSets, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("get logs", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceLogs, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionGet, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceLogs, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionGet, rbacpolicy.ResourceLogs, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})

	t.Run("create exec", func(t *testing.T) {
		modification, err := getModification("set", rbacpolicy.ResourceExec, "*", "allow")
		require.NoError(t, err)
		err = updateProjects(ctx, projIf, "foo", "test", rbacpolicy.ActionCreate, modification, false)
		require.NoError(t, err)

		projectPolicies := getProjectPolicies(ctx, projIf, "*")
		ok := checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceExec, "*/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.False(t, ok)

		projectPolicies = getProjectPolicies(ctx, projIf, "foo")
		ok = checkPolicy("foo", "role:user", rbacpolicy.ActionCreate, rbacpolicy.ResourceExec, "foo/*", assets.BuiltinPolicyCSV, uPol, projectPolicies, dRole, "glob", true, nil)
		require.True(t, ok)
	})
}

func TestNewRBACCanCommand(t *testing.T) {
	command := NewRBACCanCommand(&settingsOpts{})

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
