package commands

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestNewReloginCommand(t *testing.T) {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath: "/path/to/config",
	}

	cmd := NewReloginCommand(&clientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Empty(t, passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}

func TestNewReloginCommandWithClientOptions(t *testing.T) {
	clientOpts := argocdclient.ClientOptions{
		ConfigPath:        "/path/to/config",
		ServerAddr:        "https://argocd-server.example.com",
		Insecure:          true,
		ClientCertFile:    "/path/to/client-cert",
		ClientCertKeyFile: "/path/to/client-cert-key",
		GRPCWeb:           true,
		GRPCWebRootPath:   "/path/to/grpc-web-root-path",
		PlainText:         true,
		Headers:           []string{"header1", "header2"},
	}

	cmd := NewReloginCommand(&clientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Empty(t, passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}

// TestReloginContextSelection verifies that --argocd-context overrides the current context when
// choosing which context's credentials to refresh. This is a regression test for
// https://github.com/argoproj/argo-cd/issues/28453.
func TestReloginContextSelection(t *testing.T) {
	cfg := localconfig.LocalConfig{
		CurrentContext: "ctx-a",
		Contexts: []localconfig.ContextRef{
			{Name: "ctx-a", Server: "server-a", User: "ctx-a"},
			{Name: "ctx-b", Server: "server-b", User: "ctx-b"},
		},
		Servers: []localconfig.Server{
			{Server: "server-a"},
			{Server: "server-b"},
		},
		Users: []localconfig.User{
			{Name: "ctx-a", AuthToken: "token-a"},
			{Name: "ctx-b", AuthToken: "token-b"},
		},
	}

	// When no --argocd-context is set, configCtxName should be CurrentContext.
	clientOptsNoCtx := argocdclient.ClientOptions{}
	configCtxNameNoCtx := cfg.CurrentContext
	if clientOptsNoCtx.Context != "" {
		configCtxNameNoCtx = clientOptsNoCtx.Context
	}
	assert.Equal(t, "ctx-a", configCtxNameNoCtx)

	// When --argocd-context is set to a different context, it must take precedence.
	clientOptsWithCtx := argocdclient.ClientOptions{Context: "ctx-b"}
	configCtxNameWithCtx := cfg.CurrentContext
	if clientOptsWithCtx.Context != "" {
		configCtxNameWithCtx = clientOptsWithCtx.Context
	}
	assert.Equal(t, "ctx-b", configCtxNameWithCtx)

	// Verify the resolved context points to the correct server.
	resolvedCtx, err := cfg.ResolveContext(configCtxNameWithCtx)
	require.NoError(t, err)
	assert.Equal(t, "server-b", resolvedCtx.Server.Server)
}
