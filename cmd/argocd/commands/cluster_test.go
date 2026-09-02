package commands

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	clustermocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster/mocks"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestNewClusterListCommand_SelectorFlag(t *testing.T) {
	cmd := NewClusterListCommand(nil)
	require.NotNil(t, cmd)

	flag := cmd.Flags().Lookup("selector")
	require.NotNil(t, flag)
	assert.Equal(t, "l", flag.Shorthand)
	assert.Empty(t, flag.DefValue)
}

func mockClusterClient(t *testing.T) *clustermocks.ClusterServiceClient {
	t.Helper()
	mockClient := clustermocks.NewClusterServiceClient(t)
	original := newClusterClient
	t.Cleanup(func() { newClusterClient = original })
	newClusterClient = func(_ *argocdclient.ClientOptions, _ *cobra.Command) (io.Closer, clusterpkg.ClusterServiceClient) {
		return io.NopCloser(nil), mockClient
	}
	return mockClient
}

// TestNewClusterListCommand_SelectorSentToServer executes the command end to end against a mocked
// ClusterServiceClient, so it covers the whole path from parsing --selector/-l through to the
// ClusterListQuery that reaches the server. A regression in that wiring would not be caught by
// asserting flag registration alone.
func TestNewClusterListCommand_SelectorSentToServer(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectedSelector string
	}{
		{"no selector given", nil, ""},
		{"equality", []string{"-l", "env=prod"}, "env=prod"},
		{"double equality", []string{"-l", "env==prod"}, "env==prod"},
		{"negation", []string{"-l", "env!=prod"}, "env!=prod"},
		{"key exists", []string{"-l", "env"}, "env"},
		{"key does not exist", []string{"-l", "!env"}, "!env"},
		{"set-based in", []string{"-l", "env in (prod,staging)"}, "env in (prod,staging)"},
		{"set-based notin", []string{"-l", "env notin (dev,staging)"}, "env notin (dev,staging)"},
		{"multiple constraints", []string{"-l", "env=prod,tier=backend"}, "env=prod,tier=backend"},
		{"long flag name", []string{"--selector", "env=prod"}, "env=prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters := mockClusterClient(t)

			var gotQuery *clusterpkg.ClusterListQuery
			clusters.On("List", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					gotQuery = args.Get(1).(*clusterpkg.ClusterListQuery)
				}).
				Return(&v1alpha1.ClusterList{}, nil).
				Once()

			cmd := NewClusterListCommand(&argocdclient.ClientOptions{})
			_, _, err := runCmd(t, cmd, append([]string{"-o", "server"}, tt.args...)...)
			require.NoError(t, err)

			require.NotNil(t, gotQuery, "expected the command to issue a List call")
			assert.Equal(t, tt.expectedSelector, gotQuery.Selector)
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
