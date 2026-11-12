package helm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexCacheIntegrationMutualTLS(t *testing.T) {
	regressionRequirePinnedHelm(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "index-cache-client"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		BasicConstraintsValid: true, IsCA: true,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	require.NoError(t, err)
	clientCertificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	clientCA := x509.NewCertPool()
	clientCA.AddCert(clientCertificate)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	fixture := &regressionChartRepo{version: "1.0.0", packages: map[string][]byte{"1.0.0": regressionPackage(t, "1.0.0")}}
	fixture.server = httptest.NewUnstartedServer(http.HandlerFunc(fixture.serveHTTP))
	fixture.server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCA}
	fixture.server.StartTLS()
	t.Cleanup(fixture.server.Close)
	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw}), 0o600))
	repo := HelmRepository{Name: "private", Repo: fixture.server.URL, Creds: HelmCreds{CAPath: caPath, CertData: certData, KeyData: keyData}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		app, err := regressionBuild(t, regressionParent(t, "@private", nil), repo, store, false)
		require.NoError(t, err, "both Go index fetching and Helm archive downloading must authenticate with the client certificate")
		regressionRender(t, app, "1.0.0")
		home := app.(*helm).cmd.helmHome
		credentials, err := filepath.Glob(filepath.Join(home, "config", "repo-credential-*"))
		require.NoError(t, err)
		require.Len(t, credentials, 2)
		for _, file := range credentials {
			info, err := os.Stat(file)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
		app.Dispose()
		_, err = os.Stat(home)
		require.ErrorIs(t, err, os.ErrNotExist, "disposing the Helm home must remove all credential files")
	}
	requests, archives := fixture.snapshot()
	require.Len(t, requests, 1, "the second authenticated build must be warm")
	require.Equal(t, 2, archives)
}
