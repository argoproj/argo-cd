package e2e

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// writeSelfSignedCert writes a new self-signed certificate for serverName to
// a temporary file and returns its path.
func writeSelfSignedCert(t *testing.T, serverName string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: serverName},
		DNSNames:     []string{serverName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
	path := filepath.Join(t.TempDir(), "cert.pem")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func TestCertGet(t *testing.T) {
	fixture.EnsureCleanState(t)
	const serverName = "cert-get.example.com"

	// The certificate ConfigMap is not reset between tests, so clean up after ourselves.
	_, err := fixture.RunCli("cert", "add-tls", serverName, "--from", writeSelfSignedCert(t, serverName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := fixture.RunCli("cert", "rm", serverName, "--prompts-enabled=false")
		assert.NoError(t, err)
	})

	output, err := fixture.RunCli("cert", "get", serverName)
	require.NoError(t, err)
	assert.Contains(t, output, "Server name:  "+serverName)
	assert.Contains(t, output, "Subject:      CN="+serverName)
	assert.Contains(t, output, "DNS names:    "+serverName)
	assert.Contains(t, output, "-----BEGIN CERTIFICATE-----")

	output, err = fixture.RunCli("cert", "get", serverName, "-o", "json")
	require.NoError(t, err)
	var certs []v1alpha1.RepositoryCertificate
	require.NoError(t, json.Unmarshal([]byte(output), &certs))
	require.Len(t, certs, 1)
	assert.Equal(t, serverName, certs[0].ServerName)
	assert.Equal(t, "https", certs[0].CertType)
	assert.Contains(t, string(certs[0].CertData), "-----BEGIN CERTIFICATE-----")

	// The server name is matched exactly, so a pattern must not match.
	_, err = fixture.RunCli("cert", "get", "cert-get.*")
	assert.ErrorContains(t, err, "NotFound")
}
