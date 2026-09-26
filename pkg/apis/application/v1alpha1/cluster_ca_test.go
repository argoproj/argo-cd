package v1alpha1

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
)

func withSystemRootCAs(t *testing.T, roots []byte) {
	t.Helper()
	previous := systemRootCAsPEM
	systemRootCAsPEM = func() []byte { return roots }
	resetDefaultCABundleTrust()
	t.Cleanup(func() {
		systemRootCAsPEM = previous
		resetDefaultCABundleTrust()
	})
}

func resetDefaultCABundleTrust() {
	defaultCABundleTrustCache.Lock()
	defer defaultCABundleTrustCache.Unlock()
	defaultCABundleTrustCache.trust = nil
}

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newTestCA(t *testing.T, cn string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCA{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func (ca *testCA) newTLSServer(t *testing.T) *httptest.Server {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func get(t *testing.T, config *rest.Config) error {
	t.Helper()
	client, err := rest.HTTPClientFor(config)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, config.Host, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func TestCluster_DefaultCABundle_AddedToSystemRoots(t *testing.T) {
	publicCA := newTestCA(t, "public-ca")
	privateCA := newTestCA(t, "private-ca")
	withSystemRootCAs(t, publicCA.pem)
	publicServer := publicCA.newTLSServer(t)
	privateServer := privateCA.newTLSServer(t)

	restConfigs := map[string]func(*Cluster) (*rest.Config, error){
		"RESTConfig":    (*Cluster).RESTConfig,
		"RawRestConfig": (*Cluster).RawRestConfig,
	}
	for name, restConfig := range restConfigs {
		t.Run(name, func(t *testing.T) {
			t.Run("a cluster without caData trusts the system roots and the default bundle", func(t *testing.T) {
				for _, server := range []*httptest.Server{publicServer, privateServer} {
					config, err := restConfig(&Cluster{Server: server.URL, DefaultCABundle: privateCA.pem})
					require.NoError(t, err)
					assert.NoError(t, get(t, config), server.URL)
				}
			})
			t.Run("a cluster with its own caData trusts only that CA", func(t *testing.T) {
				cluster := &Cluster{
					Server:          publicServer.URL,
					DefaultCABundle: privateCA.pem,
					Config:          ClusterConfig{TLSClientConfig: TLSClientConfig{CAData: privateCA.pem}},
				}
				config, err := restConfig(cluster)
				require.NoError(t, err)
				var unknownAuthority x509.UnknownAuthorityError
				assert.ErrorAs(t, get(t, config), &unknownAuthority)
			})
		})
	}
}

func TestCluster_RESTConfig_SharesDefaultCABundlePool(t *testing.T) {
	withSystemRootCAs(t, generateSelfSignedCertPEM(t, "system-root-ca"))
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	var pools []*x509.CertPool
	for _, server := range []string{"https://cluster-1", "https://cluster-2"} {
		config, err := (&Cluster{Server: server, DefaultCABundle: bytes.Clone(bundle)}).RESTConfig()
		require.NoError(t, err)
		tlsConfig, err := utilnet.TLSClientConfig(config.Transport)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig)
		pools = append(pools, tlsConfig.RootCAs)
	}
	assert.Same(t, trustForDefaultCABundle(bundle).pool, pools[0], "the REST config must use the shared pool")
	assert.Same(t, pools[0], pools[1], "clusters using the same bundle must not each parse the system roots")
}

func TestTrustForDefaultCABundle(t *testing.T) {
	roots := generateSelfSignedCertPEM(t, "system-root-ca")
	withSystemRootCAs(t, roots)
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	trust := trustForDefaultCABundle(bundle)
	assert.Equal(t, slices.Concat(roots, []byte("\n"), bundle), trust.caData)
	expectedPool := x509.NewCertPool()
	require.True(t, expectedPool.AppendCertsFromPEM(trust.caData))
	assert.True(t, trust.pool.Equal(expectedPool), "the shared pool must trust exactly the system roots and the bundle")
	assert.Same(t, trust, trustForDefaultCABundle(bytes.Clone(bundle)), "clusters using the same bundle must share one parsed pool")

	rotated := generateSelfSignedCertPEM(t, "rotated-default-ca")
	rotatedTrust := trustForDefaultCABundle(rotated)
	assert.NotSame(t, trust, rotatedTrust, "a rotated bundle must not reuse the pool of the previous one")
	assert.Equal(t, slices.Concat(roots, []byte("\n"), rotated), rotatedTrust.caData)
}

func TestLoadSystemRootCAsPEM(t *testing.T) {
	t.Run("SSL_CERT_FILE takes precedence over the default files", func(t *testing.T) {
		roots := generateSelfSignedCertPEM(t, "system-root-ca")
		file := filepath.Join(t.TempDir(), "ca.pem")
		require.NoError(t, os.WriteFile(file, roots, 0o600))
		t.Setenv("SSL_CERT_FILE", file)
		assert.Equal(t, roots, loadSystemRootCAsPEM())
	})
	t.Run("no roots when the file does not exist", func(t *testing.T) {
		t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "missing.pem"))
		assert.Nil(t, loadSystemRootCAsPEM())
	})
}
