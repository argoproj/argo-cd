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

// makeSrvTLSConfig generates a minimal *tls.Config with a self-signed certificate,
// simulating what NewServer returns when it creates its own server certificate.
func makeSrvTLSConfig(t *testing.T) *ctls.Config {
	t.Helper()
	cert, err := utilstls.GenerateX509KeyPair(utilstls.CertOptions{
		Hosts:        []string{"localhost"},
		Organization: "Test Argo CD",
	})
	require.NoError(t, err)
	return &ctls.Config{
		Certificates: []ctls.Certificate{*cert},
	}
}

func TestGenerateSelfSignedCerts(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	// The first call should generate a new certificate
	config1, err := generateSelfSignedCerts()
	require.NoError(t, err)
	assert.False(t, config1.StrictValidation)
	require.Len(t, config1.ClientCertificates, 1)
	cert1 := config1.ClientCertificates[0]
	assert.NotNil(t, cert1.Leaf)

	// Second call must return the same cached certificate.
	config2, err := generateSelfSignedCerts()
	require.NoError(t, err)
	require.Len(t, config2.ClientCertificates, 1)
	cert2 := config2.ClientCertificates[0]

	assert.Equal(t, cert1.Certificate[0], cert2.Certificate[0],
		"expected the same cached certificate on repeated calls")
}

func TestGenerateSelfSignedCerts_StrictValidationAlwaysFalse(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	for range 3 {
		cfg, err := generateSelfSignedCerts()
		require.NoError(t, err)
		assert.False(t, cfg.StrictValidation)
	}
}

func TestGenerateSelfSignedCerts_ThreadSafe(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	const goroutines = 10
	results := make(chan struct {
		cert []byte
		err  error
	}, goroutines)

	for range goroutines {
		go func() {
			config, err := generateSelfSignedCerts()
			var cert []byte
			if err == nil && len(config.ClientCertificates) > 0 {
				cert = config.ClientCertificates[0].Certificate[0]
			}
			results <- struct {
				cert []byte
				err  error
			}{cert, err}
		}()
	}

	var firstCert []byte
	for range goroutines {
		res := <-results
		require.NoError(t, res.err)
		require.NotNil(t, res.cert)
		if firstCert == nil {
			firstCert = res.cert
		} else {
			assert.Equal(t, firstCert, res.cert,
				"all goroutines must receive the same cached certificate")
		}
	}
}

func TestBuildHealthCheckTLSConfig_WithRealServerCert(t *testing.T) {
	srvTLSConfig := makeSrvTLSConfig(t)

	cfg, err := buildHealthCheckTLSConfig(srvTLSConfig)
	require.NoError(t, err)

	// Happy path: strict validation must be on, CA pool must be populated.
	assert.True(t, cfg.StrictValidation,
		"strict validation must be enabled when a real server cert is available")
	assert.NotNil(t, cfg.Certificates,
		"CA pool must be populated so the health-check client can verify the server cert")
	assert.NotEmpty(t, cfg.ClientCertificates,
		"server cert must be reused as the client cert for mTLS health checks")
}

func TestBuildHealthCheckTLSConfig_FallbackPath_StrictValidationIsFalse(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	cfg, err := buildHealthCheckTLSConfig(nil)
	require.NoError(t, err)

	assert.False(t, cfg.StrictValidation,
		"fallback path must use StrictValidation=false: no CA pool is available for a self-signed cert")
	assert.Nil(t, cfg.Certificates,
		"fallback path must not set a CA pool (none is available)")
	assert.NotEmpty(t, cfg.ClientCertificates,
		"fallback path must still provide a client cert so mTLS handshakes can proceed")
}

func TestBuildHealthCheckTLSConfig_NilServerConfig(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	cfg, err := buildHealthCheckTLSConfig(nil)
	require.NoError(t, err)

	assert.NotEmpty(t, cfg.ClientCertificates)
}

func TestBuildHealthCheckTLSConfig_EmptyServerCertificates(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	cfg, err := buildHealthCheckTLSConfig(&ctls.Config{Certificates: []ctls.Certificate{}})
	require.NoError(t, err)

	assert.False(t, cfg.StrictValidation)
	assert.Nil(t, cfg.Certificates)
	assert.NotEmpty(t, cfg.ClientCertificates)
}

func TestBuildHealthCheckTLSConfig_ClientCertMatchesServerCert(t *testing.T) {
	srvTLSConfig := makeSrvTLSConfig(t)

	cfg, err := buildHealthCheckTLSConfig(srvTLSConfig)
	require.NoError(t, err)
	require.Len(t, cfg.ClientCertificates, 1)

	assert.Equal(t,
		srvTLSConfig.Certificates[0].Certificate[0],
		cfg.ClientCertificates[0].Certificate[0],
		"client cert presented during health check must be the server's own cert")
}
