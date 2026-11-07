package localconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoreKubeConfigRoundTrip checks that serialization preserves per-context paths and older configurations.
func TestCoreKubeConfigRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "argocd-config")
	cfg := LocalConfig{
		CurrentContext: "custom",
		Servers:        []Server{{Server: "kubernetes", Core: true}},
		Users:          []User{{Name: "core"}},
		Contexts: []ContextRef{
			{Name: "custom", Server: "kubernetes", User: "core", KubeConfigPath: "/etc/rancher/k3s/k3s.yaml"},
			{Name: "default", Server: "kubernetes", User: "core"},
		},
	}
	require.NoError(t, WriteLocalConfig(cfg, configPath))
	loaded, err := ReadLocalConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	custom, err := loaded.ResolveContext("")
	require.NoError(t, err)
	assert.Equal(t, "/etc/rancher/k3s/k3s.yaml", custom.KubeConfigPath)
	standard, err := loaded.ResolveContext("default")
	require.NoError(t, err)
	assert.Empty(t, standard.KubeConfigPath)
}
