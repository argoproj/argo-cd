package commands

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
	"time"
)

func generateTestCert(t *testing.T, cn string) string {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&priv.PublicKey,
		priv,
	)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	}))

	return buf.String()
}

func TestDeduplicatePEMCertificates_DuplicateCerts(t *testing.T) {
	cert := generateTestCert(t, "repo.example.com")

	pems := []string{cert, cert}

	unique, err := deduplicatePEMCertificates(pems)
	require.NoError(t, err)
	require.Len(t, unique, 1)
}

func TestDeduplicatePEMCertificates_SameSubjectDifferentCerts(t *testing.T) {
	cert1 := generateTestCert(t, "repo.example.com")
	cert2 := generateTestCert(t, "repo.example.com")

	pems := []string{cert1, cert2}

	unique, err := deduplicatePEMCertificates(pems)
	require.NoError(t, err)
	require.Len(t, unique, 2)
}

func TestDeduplicatePEMCertificates_InvalidCert(t *testing.T) {
	pems := []string{"not a cert"}

	_, err := deduplicatePEMCertificates(pems)
	require.Error(t, err)
}
