package apiclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateCert(t *testing.T) ([]byte, []byte) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"ArgoCD Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPem, keyPem
}

func TestNewClient_ProgrammaticCerts(t *testing.T) {
	certPem, keyPem := generateCert(t)

	t.Run("Test CA Cert PEM Data", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr:  "localhost:8080",
			CertPEMData: certPem,
		}
		c, err := NewClient(opts)
		require.NoError(t, err)
		assert.Equal(t, certPem, c.(*client).CertPEMData)
	})

	t.Run("Test Client Cert PEM Data", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr:           "localhost:8080",
			ClientCertPEMData:    certPem,
			ClientCertKeyPEMData: keyPem,
		}
		c, err := NewClient(opts)
		require.NoError(t, err)
		assert.NotNil(t, c.(*client).ClientCert)
	})

	t.Run("Test Client Cert PEM Data Mismatch", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr:        "localhost:8080",
			ClientCertPEMData: certPem,
		}
		_, err := NewClient(opts)
		assert.ErrorContains(t, err, "ClientCertPEMData and ClientCertKeyPEMData must always be specified together")
	})
}
