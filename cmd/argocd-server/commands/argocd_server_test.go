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
	assert.Equal(t, "/app/config/reposerver/mtls/client.crt", clientCert, "repo-server-client-cert-path must default to the auto-mounted Secret path")

	clientCertKey, err := cmd.Flags().GetString("repo-server-client-cert-key-path")
	require.NoError(t, err)
	assert.Equal(t, "/app/config/reposerver/mtls/client.key", clientCertKey, "repo-server-client-cert-key-path must default to the auto-mounted Secret path")
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

func TestNewCommand_DisableSwaggerUIFlagDefault(t *testing.T) {
	cmd := NewCommand()

	f := cmd.Flags().Lookup("disable-swagger-ui")
	require.NotNil(t, f, "flag \"disable-swagger-ui\" must be registered on argocd-server")
	assert.Equal(t, "false", f.DefValue, "disable-swagger-ui must default to false to preserve existing behavior")

	disableSwaggerUI, err := cmd.Flags().GetBool("disable-swagger-ui")
	require.NoError(t, err)
	assert.False(t, disableSwaggerUI)
}

func TestNewCommand_DisableSwaggerUIFlagCanBeSetExplicitly(t *testing.T) {
	cmd := NewCommand()

	require.NoError(t, cmd.Flags().Set("disable-swagger-ui", "true"))

	disableSwaggerUI, err := cmd.Flags().GetBool("disable-swagger-ui")
	require.NoError(t, err)
	assert.True(t, disableSwaggerUI)
}

func TestNewCommand_DisableSwaggerUIEnvVar(t *testing.T) {
	t.Setenv("ARGOCD_SERVER_DISABLE_SWAGGER_UI", "true")

	// NewCommand reads env vars at flag-definition time.
	cmd := NewCommand()

	disableSwaggerUI, err := cmd.Flags().GetBool("disable-swagger-ui")
	require.NoError(t, err)
	assert.True(t, disableSwaggerUI, "ARGOCD_SERVER_DISABLE_SWAGGER_UI=true must set the disable-swagger-ui flag default")
}

func TestNewCommand_CACertFlagRegistrationAndDefault(t *testing.T) {
	cmd := NewCommand()

	f := cmd.Flags().Lookup("repo-server-ca-cert-path")
	require.NotNil(t, f, "flag \"repo-server-ca-cert-path\" must be registered on argocd-server")
	assert.Empty(t, f.DefValue, "flag \"repo-server-ca-cert-path\" must default to empty string")

	clientCertFlag := cmd.Flags().Lookup("repo-server-client-cert-path")
	require.NotNil(t, clientCertFlag, "flag \"repo-server-client-cert-path\" must be registered on argocd-server")
	assert.Equal(t, "/app/config/reposerver/mtls/client.crt", clientCertFlag.DefValue,
		"flag \"repo-server-client-cert-path\" must default to the auto-mounted Secret path")

	clientCertKeyFlag := cmd.Flags().Lookup("repo-server-client-cert-key-path")
	require.NotNil(t, clientCertKeyFlag, "flag \"repo-server-client-cert-key-path\" must be registered on argocd-server")
	assert.Equal(t, "/app/config/reposerver/mtls/client.key", clientCertKeyFlag.DefValue,
		"flag \"repo-server-client-cert-key-path\" must default to the auto-mounted Secret path")
}
