package commands

import (
	ctls "crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
