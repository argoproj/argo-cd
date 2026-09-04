package headless

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/cache"
)

func TestDoLazy_ExternalRedisConnection(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	// Set ARGOCD_REDIS_SERVER to simulate external Redis (miniredis)
	t.Setenv("ARGOCD_REDIS_SERVER", mr.Addr())

	client := &forwardCacheClient{
		compression: cache.RedisCompressionNone,
	}

	err = client.doLazy(func(c cache.CacheClient) error {
		assert.NotNil(t, c)
		return nil
	})

	require.NoError(t, err)
	assert.NotNil(t, client.client)
}

func TestDoLazy_FallbackPath(t *testing.T) {
	// Just in case
	t.Setenv("ARGOCD_REDIS_SERVER", "")

	client := &forwardCacheClient{
		context: "invalid-context",
	}

	err := client.doLazy(func(_ cache.CacheClient) error {
		return nil
	})

	// Verify failure in finding Kubernetes context, confirming the fallback logic attempted to use in cluster Redis discovery.
	require.Error(t, err)
}

func TestDoLazy_CacheOperations(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	t.Setenv("ARGOCD_REDIS_SERVER", mr.Addr())

	client := &forwardCacheClient{}

	err = client.doLazy(func(cacheClient cache.CacheClient) error {
		testKey := "argocd-test"
		testVal := "hello-argo"

		// Verify basic cache operations with the external Redis client
		err := cacheClient.Set(&cache.Item{Key: testKey, Object: testVal})
		require.NoError(t, err)

		var result string
		err = cacheClient.Get(testKey, &result)
		require.NoError(t, err)
		assert.Equal(t, testVal, result)

		err = cacheClient.Delete(testKey)
		require.NoError(t, err)

		err = cacheClient.Get(testKey, &result)
		require.Error(t, err)

		return nil
	})

	require.NoError(t, err)
}

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
