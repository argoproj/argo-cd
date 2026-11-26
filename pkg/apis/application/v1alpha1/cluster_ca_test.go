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
	defaultCABundleTrustCache.bundleOnly = nil
	defaultCABundleTrustCache.withSystemRoots = nil
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

func TestCluster_DefaultCABundle_CATrust(t *testing.T) {
	publicCA := newTestCA(t, "public-ca")
	privateCA := newTestCA(t, "private-ca")
	withSystemRootCAs(t, publicCA.pem)
	publicServer := publicCA.newTLSServer(t)
	privateServer := privateCA.newTLSServer(t)

	tests := []struct {
		name          string
		caTrust       ClusterCATrust
		caData        []byte
		trustsPublic  bool
		trustsPrivate bool
	}{
		{name: "unset trusts only the default bundle", trustsPrivate: true},
		{name: "bundle trusts only the default bundle", caTrust: ClusterCATrustBundle, trustsPrivate: true},
		{name: "system ignores the default bundle", caTrust: ClusterCATrustSystem},
		{name: "bundle-and-system trusts the system roots and the default bundle", caTrust: ClusterCATrustBundleAndSystem, trustsPublic: true, trustsPrivate: true},
		{name: "a cluster with its own caData trusts only that CA", caTrust: ClusterCATrustBundleAndSystem, caData: publicCA.pem, trustsPublic: true},
	}
	restConfigs := map[string]func(*Cluster) (*rest.Config, error){
		"RESTConfig":    (*Cluster).RESTConfig,
		"RawRestConfig": (*Cluster).RawRestConfig,
	}
	for name, restConfig := range restConfigs {
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					for server, trusted := range map[*httptest.Server]bool{publicServer: tt.trustsPublic, privateServer: tt.trustsPrivate} {
						config, err := restConfig(&Cluster{
							Server:          server.URL,
							DefaultCABundle: privateCA.pem,
							Config:          ClusterConfig{TLSClientConfig: TLSClientConfig{CAData: tt.caData, CATrust: tt.caTrust}},
						})
						require.NoError(t, err)
						if trusted {
							assert.NoError(t, get(t, config), server.URL)
						} else {
							var verificationErr *tls.CertificateVerificationError
							assert.ErrorAs(t, get(t, config), &verificationErr, server.URL)
						}
					}
				})
			}
		})
	}
}

func TestCluster_RESTConfig_SharesDefaultCABundlePool(t *testing.T) {
	withSystemRootCAs(t, generateSelfSignedCertPEM(t, "system-root-ca"))
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	poolsByTrust := map[ClusterCATrust][]*x509.CertPool{}
	for _, caTrust := range []ClusterCATrust{"", ClusterCATrustBundle, ClusterCATrustBundleAndSystem} {
		for _, server := range []string{"https://cluster-1", "https://cluster-2"} {
			cluster := &Cluster{Server: server, DefaultCABundle: bytes.Clone(bundle), Config: ClusterConfig{TLSClientConfig: TLSClientConfig{CATrust: caTrust}}}
			config, err := cluster.RESTConfig()
			require.NoError(t, err)
			tlsConfig, err := utilnet.TLSClientConfig(config.Transport)
			require.NoError(t, err)
			require.NotNil(t, tlsConfig)
			poolsByTrust[caTrust] = append(poolsByTrust[caTrust], tlsConfig.RootCAs)
		}
	}
	assert.Same(t, trustForDefaultCABundle(bundle, false).pool, poolsByTrust[""][0], "the REST config must use the shared pool")
	assert.Same(t, poolsByTrust[""][0], poolsByTrust[""][1], "clusters using the same bundle must not each parse it")
	assert.Same(t, poolsByTrust[""][0], poolsByTrust[ClusterCATrustBundle][0], "unset and bundle must share the bundle only pool")
	assert.Same(t, trustForDefaultCABundle(bundle, true).pool, poolsByTrust[ClusterCATrustBundleAndSystem][0], "the REST config must use the shared pool")
	assert.Same(t, poolsByTrust[ClusterCATrustBundleAndSystem][0], poolsByTrust[ClusterCATrustBundleAndSystem][1], "clusters using the same bundle must not each parse the system roots")
	assert.NotSame(t, poolsByTrust[""][0], poolsByTrust[ClusterCATrustBundleAndSystem][0], "bundle only and bundle-and-system must not share a pool")
}

func TestTrustForDefaultCABundle(t *testing.T) {
	roots := generateSelfSignedCertPEM(t, "system-root-ca")
	withSystemRootCAs(t, roots)
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	for _, withSystemRoots := range []bool{false, true} {
		expectedCAData := func(bundle []byte) []byte {
			if withSystemRoots {
				return slices.Concat(roots, []byte("\n"), bundle)
			}
			return bundle
		}
		trust := trustForDefaultCABundle(bundle, withSystemRoots)
		assert.Equal(t, expectedCAData(bundle), trust.caData)
		expectedPool := x509.NewCertPool()
		require.True(t, expectedPool.AppendCertsFromPEM(trust.caData))
		assert.True(t, trust.pool.Equal(expectedPool), "the shared pool must trust exactly the expected CAs")
		assert.Same(t, trust, trustForDefaultCABundle(bytes.Clone(bundle), withSystemRoots), "clusters using the same bundle must share one parsed pool")

		rotated := generateSelfSignedCertPEM(t, "rotated-default-ca")
		rotatedTrust := trustForDefaultCABundle(rotated, withSystemRoots)
		assert.NotSame(t, trust, rotatedTrust, "a rotated bundle must not reuse the pool of the previous one")
		assert.Equal(t, expectedCAData(rotated), rotatedTrust.caData)
	}
}

func TestTrustForDefaultCABundle_ModesAreCachedSeparately(t *testing.T) {
	withSystemRootCAs(t, generateSelfSignedCertPEM(t, "system-root-ca"))
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	bundleOnly := trustForDefaultCABundle(bundle, false)
	withSystemRoots := trustForDefaultCABundle(bundle, true)
	assert.NotSame(t, bundleOnly, withSystemRoots)
	assert.Same(t, bundleOnly, trustForDefaultCABundle(bundle, false))
	assert.Same(t, withSystemRoots, trustForDefaultCABundle(bundle, true))
}

func TestClusterCATrust_Validate(t *testing.T) {
	for _, caTrust := range []ClusterCATrust{"", ClusterCATrustBundle, ClusterCATrustSystem, ClusterCATrustBundleAndSystem} {
		require.NoError(t, caTrust.Validate(), caTrust)
	}
	for _, caTrust := range []ClusterCATrust{"Bundle", "system ", "both"} {
		assert.ErrorContains(t, caTrust.Validate(), "invalid caTrust", caTrust)
	}
}

func TestCluster_DependsOnDefaultCABundle(t *testing.T) {
	tests := []struct {
		name     string
		cluster  Cluster
		expected bool
	}{
		{name: "unset", cluster: Cluster{Server: "https://1.2.3.4"}, expected: true},
		{name: "bundle", cluster: Cluster{Server: "https://1.2.3.4", Config: ClusterConfig{TLSClientConfig: TLSClientConfig{CATrust: ClusterCATrustBundle}}}, expected: true},
		{name: "bundle-and-system", cluster: Cluster{Server: "https://1.2.3.4", Config: ClusterConfig{TLSClientConfig: TLSClientConfig{CATrust: ClusterCATrustBundleAndSystem}}}, expected: true},
		{name: "system", cluster: Cluster{Server: "https://1.2.3.4", Config: ClusterConfig{TLSClientConfig: TLSClientConfig{CATrust: ClusterCATrustSystem}}}},
		{name: "own caData", cluster: Cluster{Server: "https://1.2.3.4", Config: ClusterConfig{TLSClientConfig: TLSClientConfig{CAData: []byte("ca")}}}},
		{name: "insecure", cluster: Cluster{Server: "https://1.2.3.4", Config: ClusterConfig{TLSClientConfig: TLSClientConfig{Insecure: true}}}},
		{name: "in-cluster", cluster: Cluster{Server: KubernetesInternalAPIServerAddr}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cluster.DependsOnDefaultCABundle())
		})
	}
}

func TestLoadSystemRootCAsPEM(t *testing.T) {
	writeFile := func(t *testing.T, path string, data []byte) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, data, 0o600))
	}

	t.Run("SSL_CERT_FILE takes precedence over the default files", func(t *testing.T) {
		roots := generateSelfSignedCertPEM(t, "system-root-ca")
		file := filepath.Join(t.TempDir(), "ca.pem")
		writeFile(t, file, roots)
		t.Setenv("SSL_CERT_FILE", file)
		t.Setenv("SSL_CERT_DIR", t.TempDir())
		assert.Equal(t, roots, loadSystemRootCAsPEM())
	})
	t.Run("certificates in the SSL_CERT_DIR directories are added to the file", func(t *testing.T) {
		fileRoot := generateSelfSignedCertPEM(t, "file-root-ca")
		firstDirRoot := generateSelfSignedCertPEM(t, "first-dir-root-ca")
		secondDirRoot := generateSelfSignedCertPEM(t, "second-dir-root-ca")
		file := filepath.Join(t.TempDir(), "ca.pem")
		writeFile(t, file, fileRoot)
		firstDir, secondDir := t.TempDir(), t.TempDir()
		writeFile(t, filepath.Join(firstDir, "root.pem"), firstDirRoot)
		require.NoError(t, os.Mkdir(filepath.Join(firstDir, "subdir"), 0o700))
		writeFile(t, filepath.Join(secondDir, "root.pem"), secondDirRoot)
		t.Setenv("SSL_CERT_FILE", file)
		t.Setenv("SSL_CERT_DIR", firstDir+string(os.PathListSeparator)+secondDir)

		assert.Equal(t, slices.Concat(fileRoot, firstDirRoot, secondDirRoot), loadSystemRootCAsPEM())
	})
	t.Run("a certificate found in the file and in a directory is added once", func(t *testing.T) {
		root := generateSelfSignedCertPEM(t, "system-root-ca")
		dir := t.TempDir()
		file := filepath.Join(dir, "ca-certificates.crt")
		writeFile(t, file, root)
		writeFile(t, filepath.Join(dir, "root.pem"), root)
		t.Setenv("SSL_CERT_FILE", file)
		t.Setenv("SSL_CERT_DIR", dir)

		assert.Equal(t, root, loadSystemRootCAsPEM())
	})
	t.Run("only valid certificates are kept", func(t *testing.T) {
		root := generateSelfSignedCertPEM(t, "system-root-ca")
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "README"), []byte("not a certificate"))
		writeFile(t, filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("key")}))
		writeFile(t, filepath.Join(dir, "invalid.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")}))
		writeFile(t, filepath.Join(dir, "root.pem"), root)
		t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "missing.pem"))
		t.Setenv("SSL_CERT_DIR", dir)

		assert.Equal(t, root, loadSystemRootCAsPEM())
	})
	t.Run("no roots when neither the file nor a directory has any", func(t *testing.T) {
		t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "missing.pem"))
		t.Setenv("SSL_CERT_DIR", filepath.Join(t.TempDir(), "missing"))
		assert.Nil(t, loadSystemRootCAsPEM())
	})
}

func TestCluster_RawRestConfig_ExecProviderClusterInfoWithSystemRoots(t *testing.T) {
	withSystemRootCAs(t, generateSelfSignedCertPEM(t, "system-root-ca"))
	bundle := generateSelfSignedCertPEM(t, "default-ca")

	tests := []struct {
		name               string
		caTrust            ClusterCATrust
		caData             []byte
		bundle             []byte
		provideClusterInfo bool
		expectErr          bool
	}{
		{name: "bundle-and-system with cluster info", caTrust: ClusterCATrustBundleAndSystem, bundle: bundle, provideClusterInfo: true, expectErr: true},
		{name: "bundle-and-system without cluster info", caTrust: ClusterCATrustBundleAndSystem, bundle: bundle},
		{name: "bundle-and-system with cluster info and no bundle", caTrust: ClusterCATrustBundleAndSystem, provideClusterInfo: true},
		{name: "bundle-and-system with cluster info and own caData", caTrust: ClusterCATrustBundleAndSystem, caData: bundle, bundle: bundle, provideClusterInfo: true},
		{name: "unset with cluster info", bundle: bundle, provideClusterInfo: true},
		{name: "bundle with cluster info", caTrust: ClusterCATrustBundle, bundle: bundle, provideClusterInfo: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &Cluster{
				Server:          "https://1.2.3.4",
				DefaultCABundle: tt.bundle,
				Config: ClusterConfig{
					TLSClientConfig: TLSClientConfig{CATrust: tt.caTrust, CAData: tt.caData},
					ExecProviderConfig: &ExecProviderConfig{
						APIVersion:         "client.authentication.k8s.io/v1beta1",
						Command:            "auth-plugin",
						ProvideClusterInfo: tt.provideClusterInfo,
					},
				},
			}
			_, err := cluster.RawRestConfig()
			if tt.expectErr {
				require.ErrorContains(t, err, "provideClusterInfo")
			} else {
				require.NoError(t, err)
			}
			_, err = cluster.RESTConfig()
			require.NoError(t, err)
		})
	}
}
