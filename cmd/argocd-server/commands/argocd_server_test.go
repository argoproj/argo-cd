package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommand_RegistersMTLSFlags(t *testing.T) {
	cmd := NewCommand()

	for _, flagName := range []string{
		"repo-server-ca-cert-path",
		"repo-server-client-cert-path",
		"repo-server-client-cert-key-path",
	} {
		assert.NotNilf(t, cmd.Flags().Lookup(flagName),
			"expected flag %q to be registered on argocd-server command", flagName)
	}
}

func TestNewCommand_MTLSFlagDefaults(t *testing.T) {
	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert-path")
	require.NoError(t, err)
	assert.Empty(t, caCert, "repo-server-ca-cert-path default must be empty")

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert-path")
	require.NoError(t, err)
	assert.Empty(t, clientCert, "repo-server-client-cert-path default must be empty")

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key-path")
	require.NoError(t, err)
	assert.Empty(t, clientCertKey, "repo-server-client-cert-key-path default must be empty")
}

func TestNewCommand_MTLSEnvVarPrefix(t *testing.T) {
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CA_CERT_PATH", "/etc/certs/ca.crt")
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_PATH", "/etc/certs/client.crt")
	t.Setenv("ARGOCD_SERVER_REPO_SERVER_CLIENT_CERT_KEY_PATH", "/etc/certs/client.key")

	// NewCommand reads env vars at flag-definition time.
	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert-path")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/ca.crt", caCert)

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert-path")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/client.crt", clientCert)

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key-path")
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/client.key", clientCertKey)
}

func TestNewCommand_MTLSEnvVarNotOverriddenByOtherComponents(t *testing.T) {
	t.Setenv("ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_CA_CERT_PATH", "/wrong/ca.crt")

	cmd := NewCommand()

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert-path")
	require.NoError(t, err)
	assert.Empty(t, caCert,
		"APPLICATION_CONTROLLER env var must not affect argocd-server's repo-server-ca-cert-path flag")
}

func TestNewCommand_MTLSFlagsCanBeSetExplicitly(t *testing.T) {
	cmd := NewCommand()

	require.NoError(t, cmd.Flags().Set("repo-server-ca-cert-path", "/runtime/ca.crt"))
	require.NoError(t, cmd.Flags().Set("repo-server-client-cert-path", "/runtime/client.crt"))
	require.NoError(t, cmd.Flags().Set("repo-server-client-cert-key-path", "/runtime/client.key"))

	caCert, err := cmd.Flags().GetString("repo-server-ca-cert-path")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/ca.crt", caCert)

	clientCert, err := cmd.Flags().GetString("repo-server-client-cert-path")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/client.crt", clientCert)

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key-path")
	require.NoError(t, err)
	assert.Equal(t, "/runtime/client.key", clientCertKey)
}

func TestNewCommand_RepoServerCACertTakesPrecedenceOverEmbeddedCert(t *testing.T) {
	caCertPath := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")

	cmd := NewCommand()

	require.NoError(t, cmd.Flags().Set("repo-server-ca-cert-path", caCertPath))
	require.NoError(t, cmd.Flags().Set("repo-server-strict-tls", "true"))
	tlsConfigSrcFlag := cmd.Flags().Lookup("repo-server-ca-cert-path")
	require.NotNil(t, tlsConfigSrcFlag,
		"--repo-server-ca-cert-path must be registered; if missing the wiring in NewCommand() is broken")

	assert.Equal(t, caCertPath, tlsConfigSrcFlag.Value.String())
}

func TestNewCommand_StrictTLSWithoutCACertGuardIsAbsent(t *testing.T) {
	cmd := NewCommand()

	caCertFlag := cmd.Flags().Lookup("repo-server-ca-cert-path")
	require.NotNil(t, caCertFlag)
	assert.Empty(t, caCertFlag.Value.String(),
		"repo-server-ca-cert-path must default to empty so the embedded-cert block is not bypassed")

	require.NoError(t, cmd.Flags().Set("repo-server-strict-tls", "true"))

	strictFlag := cmd.Flags().Lookup("repo-server-strict-tls")
	require.NotNil(t, strictFlag)
	assert.Equal(t, "true", strictFlag.Value.String())
}

func TestNewCommand_CACertFlagRegistrationAndDefault(t *testing.T) {
	cmd := NewCommand()

	for _, flagName := range []string{
		"repo-server-ca-cert-path",
		"repo-server-client-cert-path",
		"repo-server-client-cert-key-path",
	} {
		f := cmd.Flags().Lookup(flagName)
		require.NotNilf(t, f, "flag %q must be registered on argocd-server", flagName)
		assert.Emptyf(t, f.DefValue, "flag %q must default to empty string", flagName)
	}
}
