package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
)

type coreLoginConfig struct {
	CurrentContext string `json:"current-context"`
	Contexts       []struct {
		Name       string `json:"name"`
		KubeConfig string `json:"kubeconfig"`
	} `json:"contexts"`
}

func coreLogin(t *testing.T, configPath string, args ...string) coreLoginConfig {
	t.Helper()
	cmd := NewLoginCommand(&apiclient.ClientOptions{Core: true, ConfigPath: configPath})
	cmd.SetArgs(args)
	var commandErr error
	_, err := captureStdout(func() {
		commandErr = cmd.ExecuteContext(t.Context())
	})
	require.NoError(t, err)
	require.NoError(t, commandErr)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var cfg coreLoginConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	return cfg
}

func TestLoginCoreKubeConfigPath(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute"
		if relative {
			name = "relative"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			path := filepath.Join(dir, "k3s.yaml")
			require.NoError(t, os.WriteFile(path, []byte("token: do-not-copy-this-token\n"), 0o600))
			flagPath := path
			if relative {
				flagPath = "k3s.yaml"
			}
			configPath := filepath.Join(dir, "argocd-config")
			cfg := coreLogin(t, configPath, "--name", "k3s", "--kubeconfig", flagPath)
			require.Len(t, cfg.Contexts, 1)
			assert.Equal(t, "k3s", cfg.CurrentContext)
			assert.Equal(t, path, cfg.Contexts[0].KubeConfig)
			data, err := os.ReadFile(configPath)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "do-not-copy-this-token")
		})
	}
}

func TestLoginCoreKubeConfigContexts(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "argocd-config")
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	coreLogin(t, configPath, "--name", "first", "--kubeconfig", first)
	cfg := coreLogin(t, configPath, "--name", "second", "--kubeconfig", second)
	require.Len(t, cfg.Contexts, 2)
	assert.Equal(t, "second", cfg.CurrentContext)
	assert.Equal(t, first, cfg.Contexts[0].KubeConfig)
	assert.Equal(t, second, cfg.Contexts[1].KubeConfig)
}

func TestLoginCoreWithoutKubeConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "argocd-config")
	t.Setenv("KUBECONFIG", filepath.Join(dir, "environment.yaml"))
	coreLogin(t, configPath, "--name", "core", "--kubeconfig", filepath.Join(dir, "saved.yaml"))
	cfg := coreLogin(t, configPath, "--name", "core")
	require.Len(t, cfg.Contexts, 1)
	assert.Empty(t, cfg.Contexts[0].KubeConfig)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "kubeconfig:")
}
