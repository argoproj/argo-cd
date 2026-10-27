package headless

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func writeTestKubeConfig(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	content := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: cluster
  cluster:
    server: https://%s.example.com
contexts:
- name: context
  context:
    cluster: cluster
    user: user
    namespace: argocd
current-context: context
users:
- name: user
  user:
    token: test-token
`, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestConfigureKubeConfig(t *testing.T) {
	for _, tc := range []struct {
		name           string
		context        string
		core           bool
		explicit       bool
		emptyFlag      bool
		missing        bool
		noConfig       bool
		wantPath       string
		wantCore       bool
		wantConfigHost string
	}{
		{name: "current Argo CD context supplies its path", wantPath: "first", wantCore: true, wantConfigHost: "first"},
		{name: "saved path overrides environment", context: "first", wantPath: "first", wantCore: true, wantConfigHost: "first"},
		{name: "another Argo CD context has its own path", context: "second", wantPath: "second", wantCore: true, wantConfigHost: "second"},
		{name: "explicit flag overrides saved path", context: "first", explicit: true, wantPath: "override", wantCore: true, wantConfigHost: "override"},
		{name: "explicit core flag still uses saved path", context: "first", core: true, wantPath: "first", wantCore: true, wantConfigHost: "first"},
		{name: "old core context retains environment loading", context: "default", wantCore: true, wantConfigHost: "environment"},
		{name: "regular context is not changed", context: "regular", wantConfigHost: "environment"},
		{name: "core flag without a saved core path", context: "regular", core: true, wantCore: true, wantConfigHost: "environment"},
		{name: "empty explicit flag bypasses the saved path", context: "first", explicit: true, emptyFlag: true, wantCore: true, wantConfigHost: "environment"},
		{name: "missing saved file does not fall back", context: "first", missing: true, wantPath: "first", wantCore: true},
		{name: "no Argo CD config retains environment loading", core: true, noConfig: true, wantCore: true, wantConfigHost: "environment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := map[string]string{}
			for _, name := range []string{"first", "second", "override", "environment"} {
				paths[name] = writeTestKubeConfig(t, name)
			}
			if tc.missing {
				paths["first"] = filepath.Join(t.TempDir(), "missing")
			}
			t.Setenv("KUBECONFIG", paths["environment"])
			configPath := filepath.Join(t.TempDir(), "argocd-config")
			cfg := localconfig.LocalConfig{
				CurrentContext: "first",
				Servers: []localconfig.Server{
					{Server: "kubernetes", Core: true},
					{Server: "argocd.example.com"},
				},
				Users: []localconfig.User{{Name: "user"}},
				Contexts: []localconfig.ContextRef{
					{Name: "first", Server: "kubernetes", User: "user", KubeConfigPath: paths["first"]},
					{Name: "second", Server: "kubernetes", User: "user", KubeConfigPath: paths["second"]},
					{Name: "default", Server: "kubernetes", User: "user"},
					{Name: "regular", Server: "argocd.example.com", User: "user"},
				},
			}
			if !tc.noConfig {
				require.NoError(t, localconfig.WriteLocalConfig(cfg, configPath))
			}
			original, err := os.ReadFile(configPath)
			if tc.noConfig {
				require.True(t, os.IsNotExist(err))
			} else {
				require.NoError(t, err)
			}
			cmd := &cobra.Command{}
			clientConfig := cli.AddKubectlFlagsToSet(cmd.Flags())
			if tc.explicit {
				path := paths["override"]
				if tc.emptyFlag {
					path = ""
				}
				require.NoError(t, cmd.Flags().Set("kubeconfig", path))
			}
			opts := &apiclient.ClientOptions{ConfigPath: configPath, Context: tc.context, Core: tc.core}
			require.NoError(t, ConfigureKubeConfig(opts, cmd))
			assert.Equal(t, paths[tc.wantPath], opts.KubeConfigPath)
			assert.Equal(t, tc.wantCore, opts.Core)
			assert.Equal(t, tc.explicit, cmd.Flag("kubeconfig").Changed)
			assert.Equal(t, paths["environment"], os.Getenv("KUBECONFIG"))
			restConfig, err := clientConfig.ClientConfig()
			if tc.missing {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "https://"+tc.wantConfigHost+".example.com", restConfig.Host)
			}
			after, err := os.ReadFile(configPath)
			if tc.noConfig {
				require.True(t, os.IsNotExist(err))
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, original, after, "a command-line override must not rewrite the saved context")
		})
	}
}

func TestNewClientConfigExplicitPath(t *testing.T) {
	t.Setenv("KUBECONFIG", writeTestKubeConfig(t, "environment"))
	path := writeTestKubeConfig(t, "explicit")
	config, err := newClientConfig(nil, path).ClientConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://explicit.example.com", config.Host)
	_, err = newClientConfig(nil, filepath.Join(t.TempDir(), "missing")).ClientConfig()
	require.Error(t, err)
}
