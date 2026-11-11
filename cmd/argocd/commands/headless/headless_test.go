package headless

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestKubeContextName(t *testing.T) {
	t.Run("returns KubeOverrides.CurrentContext", func(t *testing.T) {
		cmd := &cobra.Command{}
		opts := &apiclient.ClientOptions{
			KubeOverrides: &clientcmd.ConfigOverrides{
				CurrentContext: "target-context",
			},
		}

		assert.Equal(t, "target-context", resolveAndApplyKubeContext(opts, cmd))
	})

	t.Run("prefers changed context flag and updates KubeOverrides", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("context", "", "")
		require.NoError(t, cmd.Flags().Set("context", "context-flag"))

		opts := &apiclient.ClientOptions{
			KubeOverrides: &clientcmd.ConfigOverrides{
				CurrentContext: "kube-context",
			},
		}

		assert.Equal(t, "context-flag", resolveAndApplyKubeContext(opts, cmd))
		assert.Equal(t, "context-flag", opts.KubeOverrides.CurrentContext)
	})
}

func TestNewClientConfig(t *testing.T) {
	t.Run("applies KubeOverrides.CurrentContext to REST config", func(t *testing.T) {
		tempDir := t.TempDir()
		kubeconfigPath := filepath.Join(tempDir, "kubeconfig")
		kubeconfigContent := `
apiVersion: v1
kind: Config
clusters:
- name: current-cluster
  cluster:
    server: https://current.example.com
    insecure-skip-tls-verify: true
- name: target-cluster
  cluster:
    server: https://target.example.com
    insecure-skip-tls-verify: true
contexts:
- name: current-context
  context:
    cluster: current-cluster
    user: current-user
    namespace: current-ns
- name: target-context
  context:
    cluster: target-cluster
    user: target-user
    namespace: target-ns
current-context: current-context
users:
- name: current-user
  user:
    token: current-token
- name: target-user
  user:
    token: target-token
`
		err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0o600)
		require.NoError(t, err)

		t.Setenv("KUBECONFIG", kubeconfigPath)

		clientConfig := newClientConfig(&clientcmd.ConfigOverrides{
			CurrentContext: "target-context",
		})
		require.NotNil(t, clientConfig)

		restConfig, err := clientConfig.ClientConfig()
		require.NoError(t, err)
		assert.Equal(t, "https://target.example.com", restConfig.Host)
		assert.Equal(t, "target-token", restConfig.BearerToken)

		namespace, _, err := clientConfig.Namespace()
		require.NoError(t, err)
		assert.Equal(t, "target-ns", namespace)
	})
}

func TestIsCoreMode(t *testing.T) {
	writeConfig := func(t *testing.T) string {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), "config")
		err := localconfig.WriteLocalConfig(localconfig.LocalConfig{
			CurrentContext: "kubernetes",
			Contexts: []localconfig.ContextRef{
				{Name: "kubernetes", Server: "kubernetes", User: "kubernetes"},
				{Name: "argocd.example.com", Server: "argocd.example.com", User: "argocd.example.com"},
			},
			Servers: []localconfig.Server{
				{Server: "kubernetes", Core: true},
				{Server: "argocd.example.com"},
			},
			Users: []localconfig.User{{Name: "kubernetes"}, {Name: "argocd.example.com"}},
		}, configPath)
		require.NoError(t, err)
		return configPath
	}

	t.Run("core flag", func(t *testing.T) {
		core, err := IsCoreMode(&apiclient.ClientOptions{Core: true})
		require.NoError(t, err)
		assert.True(t, core)
	})

	t.Run("no local config", func(t *testing.T) {
		core, err := IsCoreMode(&apiclient.ClientOptions{ConfigPath: filepath.Join(t.TempDir(), "missing")})
		require.NoError(t, err)
		assert.False(t, core)
	})

	t.Run("current context is core", func(t *testing.T) {
		core, err := IsCoreMode(&apiclient.ClientOptions{ConfigPath: writeConfig(t)})
		require.NoError(t, err)
		assert.True(t, core)
	})

	t.Run("selected context is not core", func(t *testing.T) {
		core, err := IsCoreMode(&apiclient.ClientOptions{ConfigPath: writeConfig(t), Context: "argocd.example.com"})
		require.NoError(t, err)
		assert.False(t, core)
	})

	t.Run("unknown context", func(t *testing.T) {
		_, err := IsCoreMode(&apiclient.ClientOptions{ConfigPath: writeConfig(t), Context: "missing"})
		require.Error(t, err)
	})
}
