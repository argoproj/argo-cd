package commands

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestDeduplicatePEMCertificates_EmptyInput(t *testing.T) {
	unique, err := deduplicatePEMCertificates([]string{})
	require.NoError(t, err)
	require.Empty(t, unique)
}

func TestDeduplicatePEMCertificates_LargeNumberOfCerts(t *testing.T) {
	const numCerts = 100
	pems := make([]string, numCerts)
	for i := range numCerts {
		pems[i] = generateTestCert(t, fmt.Sprintf("host%d.example.com", i))
	}
	unique, err := deduplicatePEMCertificates(pems)
	require.NoError(t, err)
	require.Len(t, unique, numCerts)
}

func TestDeduplicatePEMCertificates_MixedValidAndInvalid(t *testing.T) {
	cert1 := generateTestCert(t, "valid1.example.com")
	cert2 := generateTestCert(t, "valid2.example.com")
	// invalid entry before the valid ones — deduplicatePEMCertificates stops at first error
	pems := []string{"not a cert", cert1, cert2}
	_, err := deduplicatePEMCertificates(pems)
	require.Error(t, err)
	// invalid entry after some valid ones — same behaviour
	pems = []string{cert1, "not a cert", cert2}
	_, err = deduplicatePEMCertificates(pems)
	require.Error(t, err)
}
