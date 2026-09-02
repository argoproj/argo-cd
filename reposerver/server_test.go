package reposerver_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/reposerver"
	reposervercache "github.com/argoproj/argo-cd/v3/reposerver/cache"
	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/reposerver/repository"
	"github.com/argoproj/argo-cd/v3/util/askpass"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	utilstls "github.com/argoproj/argo-cd/v3/util/tls"
)

func TestNewServer_DisableTLS(t *testing.T) {
	srv, err := newTestServer(t, "", true)
	require.NoError(t, err)
	require.NotNil(t, srv)

	assert.Nil(t, srv.GetTLSConfig(), "TLS config must be nil when --disable-tls is set")
	assert.Nil(t, srv.GetHealthCheckClientCert(), "health-check client cert must be nil when TLS is disabled")
}

func TestNewServer_TLSOnly_NoMTLS(t *testing.T) {
	srv, err := newTestServer(t, "", false)
	require.NoError(t, err)
	require.NotNil(t, srv)

	tlsCfg := srv.GetTLSConfig()
	require.NotNil(t, tlsCfg, "TLS config must be non-nil when TLS is enabled")

	assert.Nil(t, srv.GetHealthCheckClientCert(), "health-check client cert must be nil when clientCAPath is not set")
	assert.NotEqual(t, tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth, "ClientAuth must not be RequireAndVerifyClientCert when mTLS is not configured")
	assert.Nil(t, tlsCfg.ClientCAs, "ClientCAs pool must be nil when no client CA path is provided")
}

func TestNewServer_MTLS_HealthCheckCertGenerated(t *testing.T) {
	caPath := writeTempCACert(t)

	srv, err := newTestServer(t, caPath, false)
	require.NoError(t, err)
	require.NotNil(t, srv)

	hcCert := srv.GetHealthCheckClientCert()
	require.NotNil(t, hcCert, "an ephemeral health-check client cert must be generated when mTLS is enabled")
	require.NotEmpty(t, hcCert.Certificate, "the health-check cert must contain at least one DER-encoded certificate")
}

func TestNewServer_MTLS_ServerRequiresClientCert(t *testing.T) {
	caPath := writeTempCACert(t)

	srv, err := newTestServer(t, caPath, false)
	require.NoError(t, err)

	tlsCfg := srv.GetTLSConfig()
	require.NotNil(t, tlsCfg)

	assert.Equal(t, tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth, "server must enforce RequireAndVerifyClientCert when a client CA path is set")
	assert.NotNil(t, tlsCfg.ClientCAs, "ClientCAs pool must be populated from the provided client CA path")
}

func TestNewServer_MTLS_HealthCheckCertRegisteredInClientCAs(t *testing.T) {
	caPath := writeTempCACert(t)

	srv, err := newTestServer(t, caPath, false)
	require.NoError(t, err)

	hcCert := srv.GetHealthCheckClientCert()
	require.NotNil(t, hcCert)

	tlsCfg := srv.GetTLSConfig()
	require.NotNil(t, tlsCfg)
	require.NotNil(t, tlsCfg.ClientCAs)

	parsedHCCert, err := x509.ParseCertificate(hcCert.Certificate[0])
	require.NoError(t, err)

	_, err = parsedHCCert.Verify(x509.VerifyOptions{
		Roots:     tlsCfg.ClientCAs,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "the health-check cert must be verifiable against the server's ClientCAs pool: "+
		"if this fails, the liveness probe will be rejected by its own server")
}

func TestNewServer_MTLS_HealthCheckCertProperties(t *testing.T) {
	caPath := writeTempCACert(t)

	srv, err := newTestServer(t, caPath, false)
	require.NoError(t, err)

	hcCert := srv.GetHealthCheckClientCert()
	require.NotNil(t, hcCert)

	parsed, err := x509.ParseCertificate(hcCert.Certificate[0])
	require.NoError(t, err)

	assert.True(t, parsed.IsCA, "health-check cert must be a CA cert so it can be added to the ClientCAs pool")
	assert.Equal(t, parsed.Subject.String(), parsed.Issuer.String(), "health-check cert must be self-signed")
	require.Len(t, parsed.ExtKeyUsage, 1, "health-check cert must carry exactly one extended key usage")
	assert.Equal(t, x509.ExtKeyUsageClientAuth, parsed.ExtKeyUsage[0], "health-check cert must carry ExtKeyUsageClientAuth, not ExtKeyUsageServerAuth")
}

func TestNewServer_MTLS_EachStartupGeneratesDistinctCert(t *testing.T) {
	caPath1 := writeTempCACert(t)
	caPath2 := writeTempCACert(t)

	srv1, err := newTestServer(t, caPath1, false)
	require.NoError(t, err)

	srv2, err := newTestServer(t, caPath2, false)
	require.NoError(t, err)

	cert1 := srv1.GetHealthCheckClientCert()
	cert2 := srv2.GetHealthCheckClientCert()

	require.NotNil(t, cert1)
	require.NotNil(t, cert2)

	assert.NotEqual(t, cert1.Certificate[0], cert2.Certificate[0], "each server instance must generate its own unique ephemeral health-check cert")
}

func TestNewServer_MTLS_InvalidCAPath(t *testing.T) {
	srv, err := newTestServer(t, "/nonexistent/path/ca.crt", false)
	require.NoError(t, err)
	require.NotNil(t, srv)

	tlsCfg := srv.GetTLSConfig()
	require.NotNil(t, tlsCfg, "TLS config must still be non-nil (server TLS is active, just no mTLS)")
	assert.Nil(t, tlsCfg.ClientCAs, "ClientCAs must be nil when the CA file does not exist")
	assert.NotEqual(t, tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth, "ClientAuth must not require client certs when CA file is absent")
	assert.Nil(t, srv.GetHealthCheckClientCert(), "no health-check cert should be generated when mTLS is skipped")
}

func TestNewServer_MTLS_InvalidCACertContent(t *testing.T) {
	invalidCAPath := filepath.Join(t.TempDir(), "bad-ca.crt")
	require.NoError(t, os.WriteFile(invalidCAPath, []byte("not a valid certificate"), 0o600))

	_, err := newTestServer(t, invalidCAPath, false)
	assert.ErrorContains(t, err, "invalid cert data")
}

func newTestServer(t *testing.T, clientCAPath string, disableTLS bool) (*reposerver.ArgoCDRepoServer, error) {
	t.Helper()

	metricsServer := metrics.NewMetricsServer()

	cache := reposervercache.NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
		1*time.Minute,
		1*time.Minute,
		10*time.Second,
	)

	askPassServer := askpass.NewServer(filepath.Join(t.TempDir(), "argocd-askpass.sock"))

	return reposerver.NewServer(
		metricsServer,
		cache,
		nil,
		repository.RepoServerInitConstants{},
		askPassServer,
		clientCAPath,
		disableTLS,
	)
}

func writeTempCACert(t *testing.T) string {
	t.Helper()

	cert, err := utilstls.GenerateX509KeyPair(utilstls.CertOptions{
		Hosts:        []string{"localhost"},
		Organization: "Argo CD Test CA",
		IsCA:         true,
		ECDSACurve:   "P256",
	})
	require.NoError(t, err)

	certPEM, _ := utilstls.EncodeX509KeyPair(*cert)

	path := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(path, certPEM, 0o600))

	return path
}
