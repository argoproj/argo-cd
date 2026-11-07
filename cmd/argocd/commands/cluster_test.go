package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/initialize"
	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/localconfig"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// TestClusterAddKubeConfigIsolation keeps registration and core management cluster configuration separate.
func TestClusterAddKubeConfigIsolation(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "default core configuration"
		if saved {
			name = "saved core configuration"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig := func(name string) string {
				path := filepath.Join(dir, name+".yaml")
				require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
					Clusters:       map[string]*clientcmdapi.Cluster{"cluster": {Server: "https://" + name + ".example.com"}},
					Contexts:       map[string]*clientcmdapi.Context{name: {Cluster: "cluster", AuthInfo: "user", Namespace: "argocd"}},
					AuthInfos:      map[string]*clientcmdapi.AuthInfo{"user": {}},
					CurrentContext: name,
				}, path))
				return path
			}
			t.Setenv("KUBECONFIG", writeConfig("environment"))
			registrationPath := writeConfig("target")
			configPath := filepath.Join(dir, "argocd-config")
			wantCorePath, wantCoreHost := "", "https://environment.example.com"
			if saved {
				wantCorePath = writeConfig("core")
				wantCoreHost = "https://core.example.com"
				require.NoError(t, localconfig.WriteLocalConfig(localconfig.LocalConfig{
					CurrentContext: "core",
					Servers:        []localconfig.Server{{Server: "kubernetes", Core: true}},
					Users:          []localconfig.User{{Name: "core"}},
					Contexts:       []localconfig.ContextRef{{Name: "core", Server: "kubernetes", User: "core", KubeConfigPath: wantCorePath}},
				}, configPath))
			}
			opts := &argocdclient.ClientOptions{ConfigPath: configPath}
			paths := clientcmd.NewDefaultPathOptions()
			cmd := initialize.InitCommand(NewClusterCommand(opts, paths))
			cmd.PersistentFlags().BoolVar(&opts.Core, "core", false, "")
			add, _, err := cmd.Find([]string{"add"})
			require.NoError(t, err)
			called := false
			add.Run = nil
			add.RunE = func(c *cobra.Command, args []string) error {
				called = true
				assert.Equal(t, []string{"target"}, args)
				assert.True(t, opts.Core)
				assert.False(t, initialize.IsCoreKubeConfigFlag(c.Flag("kubeconfig")))
				require.NoError(t, headless.ConfigureKubeConfig(opts, c))
				assert.Equal(t, wantCorePath, opts.KubeConfigPath)
				assert.Equal(t, registrationPath, paths.LoadingRules.ExplicitPath)
				target, err := getRestConfig(paths, "target")
				require.NoError(t, err)
				assert.Equal(t, "https://target.example.com", target.Host)
				loading := clientcmd.NewDefaultClientConfigLoadingRules()
				loading.ExplicitPath = opts.KubeConfigPath
				management, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).ClientConfig()
				require.NoError(t, err)
				assert.Equal(t, wantCoreHost, management.Host)
				assert.NotEqual(t, target.Host, management.Host)
				return nil
			}
			cmd.SetArgs([]string{"add", "target", "--core", "--kubeconfig", registrationPath})
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			assert.True(t, called)
			assert.Equal(t, filepath.Join(dir, "environment.yaml"), os.Getenv("KUBECONFIG"))
		})
	}
}

func Test_getQueryBySelector(t *testing.T) {
	query := getQueryBySelector("my-cluster")
	assert.Equal(t, "my-cluster", query.Name)
	assert.Empty(t, query.Server)

	query = getQueryBySelector("http://my-server")
	assert.Empty(t, query.Name)
	assert.Equal(t, "http://my-server", query.Server)

	query = getQueryBySelector("https://my-server")
	assert.Empty(t, query.Name)
	assert.Equal(t, "https://my-server", query.Server)
}

func Test_printClusterTable(_ *testing.T) {
	printClusterTable([]v1alpha1.Cluster{
		{
			Server: "my-server",
			Name:   "my-name",
			Config: v1alpha1.ClusterConfig{
				Username:           "my-username",
				Password:           "my-password",
				BearerToken:        "my-bearer-token",
				TLSClientConfig:    v1alpha1.TLSClientConfig{},
				AWSAuthConfig:      nil,
				DisableCompression: false,
			},
			Info: v1alpha1.ClusterInfo{
				ConnectionState: v1alpha1.ConnectionState{
					Status:     "my-status",
					Message:    "my-message",
					ModifiedAt: &metav1.Time{},
				},
				ServerVersion: "my-version",
			},
		},
	})
}

func Test_getRestConfig(t *testing.T) {
	type args struct {
		pathOpts *clientcmd.PathOptions
		ctxName  string
	}
	pathOpts := &clientcmd.PathOptions{
		GlobalFile:   "./testdata/config",
		LoadingRules: clientcmd.NewDefaultClientConfigLoadingRules(),
	}
	tests := []struct {
		name        string
		args        args
		expected    *rest.Config
		wantErr     bool
		expectedErr string
	}{
		{
			"Load config for context successfully",
			args{
				pathOpts,
				"argocd2.example.com:443",
			},
			&rest.Config{Host: "argocd2.example.com:443"},
			false,
			"",
		},
		{
			"Load config for current-context successfully",
			args{
				pathOpts,
				"localhost:8080",
			},
			&rest.Config{Host: "localhost:8080"},
			false,
			"",
		},
		{
			"Context not found",
			args{
				pathOpts,
				"not-exist",
			},
			nil,
			true,
			"context not-exist does not exist in kubeconfig",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRestConfig(tt.args.pathOpts, tt.args.ctxName)
			if tt.wantErr {
				require.EqualError(t, err, tt.expectedErr)
			} else {
				require.NoErrorf(t, err, "An unexpected error occurred during test %s", tt.name)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestNewClusterAddCommand_ServerProxyUrlFlagRegistered(t *testing.T) {
	pathOpts := clientcmd.NewDefaultPathOptions()
	cmd := NewClusterAddCommand(&argocdclient.ClientOptions{}, pathOpts)

	flag := cmd.Flags().Lookup("server-proxy-url")
	require.NotNil(t, flag, "--server-proxy-url flag should be registered")
	assert.Empty(t, flag.DefValue, "default value should be empty string")
	assert.Contains(t, flag.Usage, "proxy", "usage should mention proxy")
}

func TestNewClusterAddCommand_QPSAndBurstFlagsRegistered(t *testing.T) {
	pathOpts := clientcmd.NewDefaultPathOptions()
	cmd := NewClusterAddCommand(&argocdclient.ClientOptions{}, pathOpts)

	qpsFlag := cmd.Flags().Lookup("k8s-client-qps")
	require.NotNil(t, qpsFlag, "--k8s-client-qps flag should be registered")
	assert.Equal(t, "0", qpsFlag.DefValue)
	assert.Contains(t, qpsFlag.Usage, "QPS")

	burstFlag := cmd.Flags().Lookup("k8s-client-burst")
	require.NotNil(t, burstFlag, "--k8s-client-burst flag should be registered")
	assert.Equal(t, "0", burstFlag.DefValue)
	assert.Contains(t, burstFlag.Usage, "Burst")
}

func TestServerProxyUrlFlagChanged_EmptyStringIsExplicit(t *testing.T) {
	pathOpts := clientcmd.NewDefaultPathOptions()

	t.Run("flag not provided: Changed() returns false", func(t *testing.T) {
		cmd := NewClusterAddCommand(&argocdclient.ClientOptions{}, pathOpts)
		assert.False(t, cmd.Flags().Changed("server-proxy-url"))
	})

	t.Run("flag set to empty string: Changed() returns true", func(t *testing.T) {
		cmd := NewClusterAddCommand(&argocdclient.ClientOptions{}, pathOpts)
		err := cmd.Flags().Set("server-proxy-url", "")
		require.NoError(t, err)
		assert.True(t, cmd.Flags().Changed("server-proxy-url"), "Changed() must return true even for an explicit empty string")
	})

	t.Run("flag set to a URL: Changed() returns true", func(t *testing.T) {
		cmd := NewClusterAddCommand(&argocdclient.ClientOptions{}, pathOpts)
		err := cmd.Flags().Set("server-proxy-url", "https://proxy.example.com:3128")
		require.NoError(t, err)
		assert.True(t, cmd.Flags().Changed("server-proxy-url"))
	})
}

func TestApplyServerProxyUrl_OverridesProxyFromKubeconfig(t *testing.T) {
	initialProxy := "socks5://localhost:1234"

	tests := []struct {
		name             string
		serverProxyUrl   string
		flagChanged      bool
		expectedProxyUrl string
	}{
		{
			name:             "flag not provided: inherits kubeconfig proxy",
			serverProxyUrl:   "",
			flagChanged:      false,
			expectedProxyUrl: initialProxy,
		},
		{
			name:             "flag set to empty string: clears proxy for server",
			serverProxyUrl:   "",
			flagChanged:      true,
			expectedProxyUrl: "",
		},
		{
			name:             "flag set to different proxy URL: uses new proxy",
			serverProxyUrl:   "http://internal-proxy.corp:3128",
			flagChanged:      true,
			expectedProxyUrl: "http://internal-proxy.corp:3128",
		},
		{
			name:             "flag set to same proxy: explicit override preserved",
			serverProxyUrl:   "socks5://localhost:1234",
			flagChanged:      true,
			expectedProxyUrl: "socks5://localhost:1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clst := &v1alpha1.Cluster{}
			clst.Config.ProxyUrl = initialProxy

			err := applyServerProxyOverride(tt.flagChanged, tt.serverProxyUrl, clst)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedProxyUrl, clst.Config.ProxyUrl)
		})
	}
}

func TestApplyServerProxyUrl_WhenKubeconfigHasNoProxy(t *testing.T) {
	conf := &rest.Config{
		Host:  "https://192.0.2.1:6443",
		Proxy: nil,
	}
	clst := cmdutil.NewCluster("test", nil, false, conf, "token", nil, nil, nil, nil)

	assert.Empty(t, clst.Config.ProxyUrl, "no proxy should be set when kubeconfig has none and flag is absent")
}

func TestApplyServerProxyUrl_SetProxyWhenKubeconfigHasNone(t *testing.T) {
	conf := &rest.Config{
		Host:  "https://192.0.2.1:6443",
		Proxy: nil,
	}
	clst := cmdutil.NewCluster("test", nil, false, conf, "token", nil, nil, nil, nil)

	proxyURL := "http://proxy:3128"
	err := applyServerProxyOverride(true, proxyURL, clst)
	require.NoError(t, err)
	assert.Equal(t, proxyURL, clst.Config.ProxyUrl)
}

func TestApplyServerProxyOverride_NoChange(t *testing.T) {
	clst := &v1alpha1.Cluster{}
	clst.Config.ProxyUrl = "keep"

	err := applyServerProxyOverride(false, "ignored", clst)
	require.NoError(t, err)
	assert.Equal(t, "keep", clst.Config.ProxyUrl, "ProxyUrl should not change when flag not provided")
}

func TestApplyServerProxyOverride_ClearValue(t *testing.T) {
	clst := &v1alpha1.Cluster{}
	clst.Config.ProxyUrl = "old"

	err := applyServerProxyOverride(true, "", clst)
	require.NoError(t, err)
	assert.Empty(t, clst.Config.ProxyUrl, "ProxyUrl should be cleared when empty string explicitly provided")
}

func TestApplyServerProxyOverride_SetValid(t *testing.T) {
	clst := &v1alpha1.Cluster{}

	err := applyServerProxyOverride(true, "http://proxy.example:8080", clst)
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example:8080", clst.Config.ProxyUrl, "ProxyUrl should be set to provided value")
}

func TestApplyServerProxyOverride_InvalidURL(t *testing.T) {
	clst := &v1alpha1.Cluster{}
	clst.Config.ProxyUrl = "old"

	err := applyServerProxyOverride(true, "http//missing-colon", clst)
	require.Error(t, err, "expected error for invalid proxy URL")
	assert.Equal(t, "old", clst.Config.ProxyUrl, "ProxyUrl should not change on error")
}
