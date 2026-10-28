package headless

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimeUtil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/cli"
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

// TestNewClientConfig verifies that Kubernetes context overrides select the cluster and namespace.
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
		}, "")
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

// writeTestKubeConfig writes an isolated kubeconfig whose API host identifies the selected file.
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

// TestConfigureKubeConfig checks saved paths, explicit overrides, defaults, and missing-file errors.
func TestConfigureKubeConfig(t *testing.T) {
	for _, tc := range []struct {
		name             string
		context          string
		core             bool
		explicit         bool
		emptyFlag        bool
		missing          bool
		noConfig         bool
		registrationFlag bool
		wantPath         string
		wantCore         bool
		wantConfigHost   string
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
		{name: "registration flag does not override saved core path", context: "first", registrationFlag: true, explicit: true, wantPath: "first", wantCore: true, wantConfigHost: "override"},
		{name: "registration flag preserves default core loading", context: "default", registrationFlag: true, explicit: true, wantCore: true, wantConfigHost: "override"},
		{name: "saved core path does not rewrite registration defaults", context: "first", registrationFlag: true, wantPath: "first", wantCore: true, wantConfigHost: "environment"},
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
			if !tc.registrationFlag {
				initialize.MarkCoreKubeConfigFlag(cmd.Flag("kubeconfig"))
			}
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

// TestNewClientConfigExplicitPath verifies explicit paths override the environment and fail when missing.
func TestNewClientConfigExplicitPath(t *testing.T) {
	t.Setenv("KUBECONFIG", writeTestKubeConfig(t, "environment"))
	path := writeTestKubeConfig(t, "explicit")
	config, err := newClientConfig(nil, path).ClientConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://explicit.example.com", config.Host)
	_, err = newClientConfig(nil, filepath.Join(t.TempDir(), "missing")).ClientConfig()
	require.Error(t, err)
}

// TestConfigureKubeConfigErrors verifies unsupported commands and configuration errors leave options unchanged.
func TestConfigureKubeConfigErrors(t *testing.T) {
	t.Run("command without a kubeconfig flag skips configuration", func(t *testing.T) {
		opts := &apiclient.ClientOptions{Core: true, ConfigPath: t.TempDir()}
		require.NoError(t, ConfigureKubeConfig(opts, &cobra.Command{}))
		assert.Empty(t, opts.KubeConfigPath)
	})
	t.Run("malformed local configuration", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config")
		require.NoError(t, os.WriteFile(path, []byte("contexts: [\n"), 0o600))
		cmd := &cobra.Command{}
		cli.AddKubectlFlagsToSet(cmd.Flags())
		initialize.MarkCoreKubeConfigFlag(cmd.Flag("kubeconfig"))
		opts := &apiclient.ClientOptions{ConfigPath: path}
		require.ErrorContains(t, ConfigureKubeConfig(opts, cmd), "error reading local config")
		assert.False(t, opts.Core)
		assert.Empty(t, opts.KubeConfigPath)
	})
	t.Run("unknown Argo CD context", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config")
		require.NoError(t, localconfig.WriteLocalConfig(localconfig.LocalConfig{}, path))
		cmd := &cobra.Command{}
		cli.AddKubectlFlagsToSet(cmd.Flags())
		initialize.MarkCoreKubeConfigFlag(cmd.Flag("kubeconfig"))
		opts := &apiclient.ClientOptions{ConfigPath: path, Context: "missing"}
		err := ConfigureKubeConfig(opts, cmd)
		require.ErrorContains(t, err, "error resolving context")
		require.ErrorContains(t, err, "missing")
		assert.False(t, opts.Core)
	})
	t.Run("flag rejects the selected path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config")
		cfg := localconfig.LocalConfig{
			CurrentContext: "core",
			Servers:        []localconfig.Server{{Server: "kubernetes", Core: true}},
			Users:          []localconfig.User{{Name: "core"}},
			Contexts:       []localconfig.ContextRef{{Name: "core", Server: "kubernetes", User: "core", KubeConfigPath: "saved.yaml"}},
		}
		require.NoError(t, localconfig.WriteLocalConfig(cfg, path))
		cmd := &cobra.Command{}
		cmd.Flags().Int("kubeconfig", 0, "")
		initialize.MarkCoreKubeConfigFlag(cmd.Flag("kubeconfig"))
		opts := &apiclient.ClientOptions{ConfigPath: path}
		require.ErrorContains(t, ConfigureKubeConfig(opts, cmd), "error configuring kubeconfig path")
		assert.False(t, opts.Core)
		assert.Empty(t, opts.KubeConfigPath)
	})
}

// TestMaybeStartLocalServerKubeConfig verifies path selection and failures before core server startup.
func TestMaybeStartLocalServerKubeConfig(t *testing.T) {
	t.Run("invalid environment path", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "original")
		opts := &apiclient.ClientOptions{Core: true, KubeConfigPath: "invalid\x00path"}
		shutdown, err := MaybeStartLocalServer(t.Context(), opts, "", nil, nil, nil)
		require.ErrorContains(t, err, "error setting kubeconfig path")
		assert.Nil(t, shutdown)
		assert.Equal(t, "original", os.Getenv("KUBECONFIG"))
	})
	t.Run("missing selected file does not fall back to environment", func(t *testing.T) {
		t.Setenv("KUBECONFIG", writeTestKubeConfig(t, "environment"))
		t.Setenv(v1alpha1.EnvVarFakeInClusterConfig, os.Getenv(v1alpha1.EnvVarFakeInClusterConfig))
		t.Setenv(common.EnvLogLevel, os.Getenv(common.EnvLogLevel))
		handlers, level := runtimeUtil.ErrorHandlers, log.GetLevel()
		t.Cleanup(func() {
			runtimeUtil.ErrorHandlers = handlers
			log.SetLevel(level)
		})
		path := filepath.Join(t.TempDir(), "missing")
		opts := &apiclient.ClientOptions{Core: true, KubeConfigPath: path}
		port := 8080
		shutdown, err := MaybeStartLocalServer(t.Context(), opts, "", &port, nil, nil)
		require.ErrorContains(t, err, "error creating client config")
		require.ErrorContains(t, err, path)
		assert.Nil(t, shutdown)
		assert.Equal(t, path, os.Getenv("KUBECONFIG"))
	})
}

// TestNewClientOrDieKubeConfigError verifies invalid CLI configuration stops client creation immediately.
func TestNewClientOrDieKubeConfigError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte("contexts: [\n"), 0o600))
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cli.AddKubectlFlagsToSet(cmd.Flags())
	initialize.MarkCoreKubeConfigFlag(cmd.Flag("kubeconfig"))
	logger := log.StandardLogger()
	exit, output := logger.ExitFunc, logger.Out
	t.Cleanup(func() {
		logger.ExitFunc = exit
		logger.SetOutput(output)
	})
	var logs bytes.Buffer
	logger.SetOutput(&logs)
	logger.ExitFunc = func(code int) { panic(code) }
	require.PanicsWithValue(t, 1, func() {
		NewClientOrDie(&apiclient.ClientOptions{ConfigPath: path}, cmd)
	})
	assert.Contains(t, logs.String(), "error reading local config")
}
