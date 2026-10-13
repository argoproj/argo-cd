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
	"net/http"
	"net/url"
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

// The UI requests certificate details through the REST API, where a colon in
// the server name must not be taken for a custom verb of the route.
func TestCertGetHTTP(t *testing.T) {
	fixture.EnsureCleanState(t)
	const (
		tlsServerName = "cert-get-http.example.com"
		sshServerName = "[cert-get-http.example.com]:2222"
		sshKey        = "AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"
	)

	_, err := fixture.RunCli("cert", "add-tls", tlsServerName, "--from", writeSelfSignedCert(t, tlsServerName))
	require.NoError(t, err)
	_, err = fixture.RunCliWithStdin(sshServerName+" ssh-ed25519 "+sshKey+"\n", false, "cert", "add-ssh", "--batch")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := fixture.RunCli("cert", "rm", tlsServerName, "--cert-type", "https", "--prompts-enabled=false")
		assert.NoError(t, err)
		_, err = fixture.RunCli("cert", "rm", sshServerName, "--cert-type", "ssh", "--prompts-enabled=false")
		assert.NoError(t, err)
	})

	tests := []struct {
		name       string
		serverName string
		certType   string
	}{
		{name: "TLS certificate", serverName: tlsServerName, certType: "https"},
		{name: "SSH entry with port", serverName: sshServerName, certType: "ssh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var certs v1alpha1.RepositoryCertificateList
			path := "/api/v1/certificates/" + url.PathEscape(tt.serverName) + "/details?certType=" + tt.certType
			require.NoError(t, fixture.DoHttpJsonRequest(http.MethodGet, path, &certs))
			require.Len(t, certs.Items, 1)
			assert.Equal(t, tt.serverName, certs.Items[0].ServerName)
			assert.NotEmpty(t, certs.Items[0].CertData)
		})
	}
}
