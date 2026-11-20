package commands

import (
	ctls "crypto/tls"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
	utilstls "github.com/argoproj/argo-cd/v3/util/tls"
)

func TestNewCommand_DisableTLSFlag(t *testing.T) {
	cmd := NewCommand()

	flag := cmd.Flags().Lookup("disable-tls")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)

	require.NoError(t, cmd.Flags().Set("disable-tls", "true"))
	value, err := cmd.Flags().GetBool("disable-tls")
	require.NoError(t, err)
	assert.True(t, value)
}

func TestNewCommand_DisableTLSAndClientCAPathAreMutuallyExclusive(t *testing.T) {
	t.Setenv("ARGOCD_EXEC_TIMEOUT", "1ms")

	cmd := NewCommand()
	cmd.SetArgs([]string{"--disable-tls", "--client-ca-path", "/tmp/client-ca.crt"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ca-path cannot be used when --disable-tls is enabled")
}

func TestBuildHealthCheckTLSConfig_NilCert(t *testing.T) {
	cfg := buildHealthCheckTLSConfig(nil, false)

	assert.False(t, cfg.StrictValidation, "strict validation must be off: the probe verifies itself over localhost")
	assert.Empty(t, cfg.ClientCertificates, "no client cert must be present when mTLS is not active")
	assert.Nil(t, cfg.Certificates, "no CA pool must be set: server-cert verification is skipped for localhost probes")
	assert.False(t, cfg.DisableTLS, "TLS itself must not be disabled: it is still a TLS connection")
}

func TestBuildHealthCheckTLSConfig_WithCert(t *testing.T) {
	hcCert, err := utilstls.GenerateHealthCheckClientCert()
	require.NoError(t, err)
	require.NotNil(t, hcCert)

	cfg := buildHealthCheckTLSConfig(hcCert, false)

	assert.False(t, cfg.StrictValidation, "server-cert verification is always skipped for localhost self-connections")
	require.Len(t, cfg.ClientCertificates, 1, "the ephemeral health-check cert must appear as the client identity")
	assert.Equal(t, hcCert.Certificate[0], cfg.ClientCertificates[0].Certificate[0], "the propagated cert bytes must match the ephemeral cert exactly")
	assert.Nil(t, cfg.Certificates, "no CA pool must be set: server-cert verification is skipped")
}

func TestBuildHealthCheckTLSConfig_IsIdempotent(t *testing.T) {
	hcCert, err := utilstls.GenerateHealthCheckClientCert()
	require.NoError(t, err)

	cfg1 := buildHealthCheckTLSConfig(hcCert, false)
	cfg2 := buildHealthCheckTLSConfig(hcCert, false)

	assert.Equal(t, cfg1.StrictValidation, cfg2.StrictValidation)
	assert.Equal(t, cfg1.DisableTLS, cfg2.DisableTLS)
	require.Len(t, cfg1.ClientCertificates, 1)
	require.Len(t, cfg2.ClientCertificates, 1)
	assert.Equal(t, cfg1.ClientCertificates[0].Certificate[0], cfg2.ClientCertificates[0].Certificate[0],
		"repeated calls must return equivalent configs with no shared mutable state")
}

func TestBuildHealthCheckTLSConfig_MutatingReturnedCertDoesNotAffectSource(t *testing.T) {
	hcCert, err := utilstls.GenerateHealthCheckClientCert()
	require.NoError(t, err)

	originalDER := make([]byte, len(hcCert.Certificate[0]))
	copy(originalDER, hcCert.Certificate[0])

	cfg := buildHealthCheckTLSConfig(hcCert, false)
	require.Len(t, cfg.ClientCertificates, 1)

	cfg.ClientCertificates[0] = ctls.Certificate{}

	assert.Equal(t, originalDER, hcCert.Certificate[0], "mutating the returned config must not affect the source certificate")
}

// TestNewCommand_OTLPHeadersFlagUsesEnv proves the otlp-headers flag default is wired to the
// canonical ARGOCD_REPO_SERVER_OTLP_HEADERS environment variable.
func TestNewCommand_OTLPHeadersFlagUsesEnv(t *testing.T) {
	t.Setenv("ARGOCD_REPO_SERVER_OTLP_HEADERS", "traceparent=abc123")

	cmd := NewCommand()

	got, err := cmd.Flags().GetStringToString("otlp-headers")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"traceparent": "abc123"}, got)
}

func TestHelmIndexCacheDirectoryFlag(t *testing.T) {
	temporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, temporary)
	}
	t.Setenv(common.EnvHelmIndexCacheEnabled, "")
	t.Setenv(common.EnvHelmIndexCacheDir, "")
	t.Setenv(common.EnvHelmIndexCacheFilterEnabled, "")
	command := NewCommand()
	directory, err := command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(temporary, "helm-index-cache"), directory, "the default must respect the system temporary directory")
	enabled, err := command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.False(t, enabled, "having a default directory must not enable dependency reuse")
	filtered, err := command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.False(t, filtered, "dependency-only snapshots require a separate opt-in")
	t.Setenv(common.EnvHelmIndexCacheEnabled, "true")
	command = NewCommand()
	enabled, err = command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.True(t, enabled)
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(temporary, "helm-index-cache"), directory, "enabling reuse must not require a directory override")
	t.Setenv(common.EnvHelmIndexCacheDir, "/tmp/private-helm-indexes")
	command = NewCommand()
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/private-helm-indexes", directory)
	require.NoError(t, command.Flags().Set("helm-index-cache-dir", "/tmp/explicit-indexes"))
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/explicit-indexes", directory)
	require.NoError(t, command.Flags().Set("helm-index-cache-enabled", "false"))
	enabled, err = command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.False(t, enabled, "the explicit flag must override environment enablement")
	t.Setenv(common.EnvHelmIndexCacheFilterEnabled, "true")
	command = NewCommand()
	filtered, err = command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.True(t, filtered)
	require.NoError(t, command.Flags().Set("helm-index-cache-filter-enabled", "false"))
	filtered, err = command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.False(t, filtered, "the explicit flag must force complete indexes despite environment enablement")
}

func TestHelmIndexCacheExpirationFlag(t *testing.T) {
	t.Setenv(common.EnvHelmIndexCacheExpiration, "")
	t.Setenv("ARGOCD_RECONCILIATION_TIMEOUT", "15m")
	command := NewCommand()
	expiration, err := command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, time.Minute, expiration, "Helm freshness must not inherit the Git revision interval")
	require.NoError(t, command.Flags().Set("revision-cache-expiration", "30m"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, time.Minute, expiration)
	t.Setenv(common.EnvHelmIndexCacheExpiration, "30s")
	command = NewCommand()
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, expiration)
	require.NoError(t, command.Flags().Set("helm-index-cache-expiration", "3m"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, 3*time.Minute, expiration, "the explicit flag must override environment freshness")
	require.NoError(t, command.Flags().Set("helm-index-cache-expiration", "0s"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Zero(t, expiration, "zero requests revalidation on every use")
}
