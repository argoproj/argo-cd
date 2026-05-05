package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommand_RegistersMTLSFlags(t *testing.T) {
	cmd := NewCommand()

	for _, flagName := range []string{
		"repo-server-ca-cert",
		"repo-server-client-cert",
		"repo-server-client-cert-key",
	} {
		assert.NotNilf(t, cmd.Flags().Lookup(flagName),
			"expected flag %q to be registered on argocd-server command", flagName)
	}
}

func TestNewCommand_MTLSFlagDefaults(t *testing.T) {
	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert")
	require.NoError(t, err)
	assert.Empty(t, caCert, "repo-server-ca-cert default must be empty")

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert")
	require.NoError(t, err)
	assert.Empty(t, clientCert, "repo-server-client-cert default must be empty")

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key")
	require.NoError(t, err)
	assert.Empty(t, clientCertKey, "repo-server-client-cert-key default must be empty")
}

func TestNewCommand_MTLSEnvVarPrefix(t *testing.T) {
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CA_CERT", "/etc/certs/ca.crt")
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT", "/etc/certs/client.crt")
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_KEY", "/etc/certs/client.key")

	// NewCommand reads env vars at flag-definition time.
	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/ca.crt", caCert)

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/client.crt", clientCert)

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/client.key", clientCertKey)
}

func TestNewCommand_MTLSEnvVarNotOverriddenByOtherComponents(t *testing.T) {
	t.Setenv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CA_CERT", "/wrong/ca.crt")

	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert")
	require.NoError(t, err)
	assert.Empty(t, caCert,
		"APPLICATION_CONTROLLER env var must not affect argocd-server's repo-server-ca-cert flag")
}

func TestNewCommand_MTLSFlagsCanBeSetExplicitly(t *testing.T) {
	cmd := NewCommand()

	require.NoError(t, cmd.Flags().Set("repo-server-ca-cert", "/runtime/ca.crt"))
	require.NoError(t, cmd.Flags().Set("repo-server-client-cert", "/runtime/client.crt"))
	require.NoError(t, cmd.Flags().Set("repo-server-client-cert-key", "/runtime/client.key"))

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/ca.crt", caCert)

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/client.crt", clientCert)

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/client.key", clientCertKey)
}
