package admin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestDashboardCommand_SignalHandling_GracefulShutdown(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	d := &dashboard{
		startLocalServer: func(_ context.Context, opts *apiclient.ClientOptions, _ string, _ *int, _ *string, _ clientcmd.ClientConfig) (func(), error) {
			assert.True(t, opts.Core, "Core client option should be set to true")
			close(started)
			return func() { close(stopped) }, nil
		},
	}

	cmd := newDashboardCommand(d, &apiclient.ClientOptions{})
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.ExecuteContext(t.Context())
	}()

	// the signal handler is registered before the command body runs, so a started server means it is in place
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: local server was never started")
	}

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(syscall.SIGINT))

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: dashboard command did not exit after SIGINT")
	}

	select {
	case <-stopped:
	default:
		t.Fatal("shutdown func was not called")
	}
}

// writeDashboardKubeConfig creates a file whose API address identifies the selected configuration.
func writeDashboardKubeConfig(t *testing.T, host string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"cluster": {Server: host}},
		Contexts:       map[string]*clientcmdapi.Context{"context": {Cluster: "cluster", AuthInfo: "user", Namespace: "argocd"}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{"user": {Token: "test-token"}},
		CurrentContext: "context",
	}, path))
	return path
}

// TestDashboardCommandKubeConfig checks saved paths, explicit overrides, and dangling CLI contexts.
func TestDashboardCommandKubeConfig(t *testing.T) {
	for _, tc := range []struct {
		name       string
		context    string
		explicit   bool
		dangling   bool
		noConfig   bool
		wantError  bool
		wantConfig string
	}{
		{name: "saved current context", wantConfig: "first"},
		{name: "selected Argo CD context", context: "second", wantConfig: "second"},
		{name: "explicit kubeconfig overrides saved path", explicit: true, wantConfig: "override"},
		{name: "dangling current context fails before startup", dangling: true, wantError: true},
		{name: "explicit kubeconfig bypasses dangling current context", dangling: true, explicit: true, wantConfig: "override"},
		{name: "no CLI config retains Kubernetes defaults", noConfig: true, wantConfig: "environment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := map[string]string{}
			for _, name := range []string{"first", "second", "override", "environment"} {
				paths[name] = writeDashboardKubeConfig(t, "https://"+name+".example.com")
			}
			t.Setenv("KUBECONFIG", paths["environment"])
			configPath := filepath.Join(t.TempDir(), "argocd-config")
			cfg := localconfig.LocalConfig{
				CurrentContext: "first",
				Servers:        []localconfig.Server{{Server: "kubernetes", Core: true}},
				Users:          []localconfig.User{{Name: "core"}},
				Contexts: []localconfig.ContextRef{
					{Name: "first", Server: "kubernetes", User: "core", KubeConfigPath: paths["first"]},
					{Name: "second", Server: "kubernetes", User: "core", KubeConfigPath: paths["second"]},
				},
			}
			if tc.dangling {
				cfg.CurrentContext = "missing"
			}
			if !tc.noConfig {
				require.NoError(t, localconfig.WriteLocalConfig(cfg, configPath))
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started, stopped := false, false
			d := &dashboard{startLocalServer: func(_ context.Context, opts *apiclient.ClientOptions, kubeContext string, _ *int, _ *string, config clientcmd.ClientConfig) (func(), error) {
				started = true
				assert.True(t, opts.Core)
				wantPath := paths[tc.wantConfig]
				if tc.noConfig {
					wantPath = ""
				}
				assert.Equal(t, wantPath, opts.KubeConfigPath)
				assert.Equal(t, "context", kubeContext)
				rest, err := config.ClientConfig()
				require.NoError(t, err)
				assert.Equal(t, "https://"+tc.wantConfig+".example.com", rest.Host)
				namespace, _, err := config.Namespace()
				require.NoError(t, err)
				assert.Equal(t, "dashboard-ns", namespace)
				cancel()
				return func() { stopped = true }, nil
			}}
			opts := &apiclient.ClientOptions{ConfigPath: configPath}
			cmd := newDashboardCommand(d, opts)
			cmd.Flags().StringVar(&opts.Context, "argocd-context", "", "")
			args := []string{"--context", "context", "--namespace", "dashboard-ns"}
			if tc.explicit {
				args = append(args, "--kubeconfig", paths["override"])
			}
			if tc.context != "" {
				args = append(args, "--argocd-context", tc.context)
			}
			cmd.SetArgs(args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.ExecuteContext(ctx)
			if tc.wantError {
				require.ErrorContains(t, err, "local config invalid")
				assert.False(t, started)
				assert.False(t, stopped)
			} else {
				require.NoError(t, err)
				assert.True(t, started)
				assert.True(t, stopped)
			}
		})
	}
}
