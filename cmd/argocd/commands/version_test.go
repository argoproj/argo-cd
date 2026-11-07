package commands

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestShortVersionClient(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version", "--short", "--client"})
	require.NoError(t, cmd.Execute(), "Failed to execute short version command")
	assert.Equal(t, "argocd: v99.99.99+unknown\n", buf.String())
}

func TestShortVersion(t *testing.T) {
	serverVersion := &version.VersionMessage{Version: "v99.99.99+unknown"}
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, serverVersion)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"argocd", "version", "--short"})
	require.NoError(t, cmd.Execute(), "Failed to execute short version command")
	assert.Equal(t, "argocd: v99.99.99+unknown\nargocd-server: v99.99.99+unknown\n", buf.String())
}

func writeCoreLocalConfig(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config")
	err := localconfig.WriteLocalConfig(localconfig.LocalConfig{
		CurrentContext: "kubernetes",
		Contexts:       []localconfig.ContextRef{{Name: "kubernetes", Server: "kubernetes", User: "kubernetes"}},
		Servers:        []localconfig.Server{{Server: "kubernetes", Core: true}},
		Users:          []localconfig.User{{Name: "kubernetes"}},
	}, configPath)
	require.NoError(t, err)
	return configPath
}

func TestVersionCoreMode(t *testing.T) {
	// In core mode the CLI talks to an in-process API server built from its own code, so whatever that server
	// reports is the CLI's version, not the version of Argo CD running in the cluster.
	cliBuiltServerVersion := &version.VersionMessage{Version: "v99.99.99+unknown", BuildDate: "2006-01-02T15:04:05Z"}

	t.Run("core flag", func(t *testing.T) {
		buf := new(bytes.Buffer)
		cmd := NewVersionCmd(&argocdclient.ClientOptions{Core: true}, cliBuiltServerVersion)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"version"})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, buf.String(), "argocd-server: unavailable in core mode\n")
		assert.NotContains(t, buf.String(), "argocd-server: v99.99.99+unknown")
		assert.NotContains(t, buf.String(), "BuildDate: 2006-01-02T15:04:05Z")
	})

	t.Run("core context in local config", func(t *testing.T) {
		buf := new(bytes.Buffer)
		cmd := NewVersionCmd(&argocdclient.ClientOptions{ConfigPath: writeCoreLocalConfig(t)}, cliBuiltServerVersion)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"version", "--short"})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, "argocd: v99.99.99+unknown\nargocd-server: unavailable in core mode\n", buf.String())
	})

	t.Run("json output keeps the server key", func(t *testing.T) {
		cmd := NewVersionCmd(&argocdclient.ClientOptions{Core: true}, cliBuiltServerVersion)
		cmd.SetArgs([]string{"version", "-o", "json"})
		output, err := captureOutput(cmd.Execute)
		require.NoError(t, err)
		var out struct {
			Server version.VersionMessage `json:"server"`
		}
		require.NoError(t, json.Unmarshal([]byte(output), &out))
		assert.Equal(t, version.VersionMessage{Version: "unavailable in core mode"}, out.Server)
	})

	t.Run("short json output keeps the argocd-server key", func(t *testing.T) {
		cmd := NewVersionCmd(&argocdclient.ClientOptions{Core: true}, cliBuiltServerVersion)
		cmd.SetArgs([]string{"version", "--short", "-o", "json"})
		output, err := captureOutput(cmd.Execute)
		require.NoError(t, err)
		var out struct {
			Server map[string]string `json:"server"`
		}
		require.NoError(t, json.Unmarshal([]byte(output), &out))
		assert.Equal(t, map[string]string{"argocd-server": "unavailable in core mode"}, out.Server)
	})

	t.Run("root command with --core does not start a local server", func(t *testing.T) {
		// No server version is injected here: the version comes from the real code path behind `argocd version
		// --core`. Without a reachable cluster, starting the in-process API server would fail.
		t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing-kubeconfig"))
		buf := new(bytes.Buffer)
		cmd := NewCommand()
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"version", "--core", "--short", "--config", filepath.Join(t.TempDir(), "config")})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, "argocd: v99.99.99+unknown\nargocd-server: unavailable in core mode\n", buf.String())
	})

	t.Run("client only", func(t *testing.T) {
		buf := new(bytes.Buffer)
		cmd := NewVersionCmd(&argocdclient.ClientOptions{Core: true}, cliBuiltServerVersion)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"version", "--short", "--client"})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, "argocd: v99.99.99+unknown\n", buf.String())
	})
}
