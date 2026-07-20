package dex

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestGenerateDexConfigYAML_WebTLSMinVersion12(t *testing.T) {
	settings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		DexConfig: `
connectors: []
web:
  tlsMinVersion: "1.2"
`,
	}

	out, err := GenerateDexConfigYAML(settings, false)
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(out, &cfg))

	webCfg, ok := cfg["web"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "0.0.0.0:5556", webCfg["https"])
	assert.Equal(t, "/tmp/tls.crt", webCfg["tlsCert"])
	assert.Equal(t, "/tmp/tls.key", webCfg["tlsKey"])
	assert.Equal(t, "1.2", webCfg["tlsMinVersion"])
}

func TestGenerateDexConfigYAML_WebTLSMinVersion13(t *testing.T) {
	settings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		DexConfig: `
connectors: []
web:
  tlsMinVersion: "1.3"
`,
	}

	out, err := GenerateDexConfigYAML(settings, false)
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(out, &cfg))

	webCfg, ok := cfg["web"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "0.0.0.0:5556", webCfg["https"])
	assert.Equal(t, "/tmp/tls.crt", webCfg["tlsCert"])
	assert.Equal(t, "/tmp/tls.key", webCfg["tlsKey"])
	assert.Equal(t, "1.3", webCfg["tlsMinVersion"])
}

func TestGenerateDexConfigYAML_WebTLSMinVersionNil(t *testing.T) {
	settings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		DexConfig: `
connectors: []
web:
  tlsMinVersion: ""
`,
	}

	out, err := GenerateDexConfigYAML(settings, false)
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(out, &cfg))

	webCfg, ok := cfg["web"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "0.0.0.0:5556", webCfg["https"])
	assert.Equal(t, "/tmp/tls.crt", webCfg["tlsCert"])
	assert.Equal(t, "/tmp/tls.key", webCfg["tlsKey"])
	_, found := webCfg["tlsMinVersion"]
	assert.False(t, found)
}

func TestGenerateDexConfigYAML_WebTLSMinVersionNoWebSettings(t *testing.T) {
	settings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		DexConfig: `
connectors: []
`,
	}

	out, err := GenerateDexConfigYAML(settings, false)
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(out, &cfg))

	webCfg, ok := cfg["web"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "0.0.0.0:5556", webCfg["https"])
	assert.Equal(t, "/tmp/tls.crt", webCfg["tlsCert"])
	assert.Equal(t, "/tmp/tls.key", webCfg["tlsKey"])
	_, found := webCfg["tlsMinVersion"]
	assert.False(t, found)
}
