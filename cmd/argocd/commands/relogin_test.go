package commands

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
)

func TestNewReloginCommand(t *testing.T) {
	globalClientOpts := argocdclient.ClientOptions{
		ConfigPath: "/path/to/config",
	}

	cmd := NewReloginCommand(&globalClientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Equal(t, "", passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}

func TestNewReloginCommandWithGlobalClientOptions(t *testing.T) {
	globalClientOpts := argocdclient.ClientOptions{
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

	cmd := NewReloginCommand(&globalClientOpts)

	assert.Equal(t, "relogin", cmd.Use, "Unexpected command Use")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Short, "Unexpected command Short")
	assert.Equal(t, "Refresh an expired authenticate token", cmd.Long, "Unexpected command Long")

	// Assert command flags
	passwordFlag := cmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag, "Expected flag --password to be defined")
	assert.Equal(t, "", passwordFlag.Value.String(), "Unexpected default value for --password flag")

	ssoPortFlag := cmd.Flags().Lookup("sso-port")
	port, err := strconv.Atoi(ssoPortFlag.Value.String())
	assert.NotNil(t, ssoPortFlag, "Expected flag --sso-port to be defined")
	require.NoError(t, err, "Failed to convert sso-port flag value to integer")
	assert.Equal(t, 8085, port, "Unexpected default value for --sso-port flag")
}
