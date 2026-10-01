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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
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

func TestPrintCertDetails_SSH(t *testing.T) {
	var out bytes.Buffer
	printCertDetails(&out, []appsv1.RepositoryCertificate{{
		ServerName:  "github.com",
		CertType:    "ssh",
		CertSubType: "ssh-ed25519",
		CertData:    []byte("AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"),
		CertInfo:    "SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
	}}, time.Now())

	assert.Contains(t, out.String(), "Fingerprint:  SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU\n")
	assert.Contains(t, out.String(), "Data:         github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n")
}

func TestPrintCertDetails_TLS(t *testing.T) {
	pemData := generateTestCert(t, "cd.example.com")
	x509Cert, err := certutil.DecodePEMCertificateToX509(pemData)
	require.NoError(t, err)
	certs := []appsv1.RepositoryCertificate{{
		ServerName:  "cd.example.com",
		CertType:    "https",
		CertSubType: "rsa",
		CertData:    []byte(pemData),
	}}

	tests := []struct {
		name     string
		now      time.Time
		validity string
	}{
		{name: "valid", now: x509Cert.NotBefore.Add(time.Minute), validity: "\n"},
		{name: "expired", now: x509Cert.NotAfter.Add(time.Minute), validity: " (expired)\n"},
		{name: "not yet valid", now: x509Cert.NotBefore.Add(-time.Minute), validity: " (not yet valid)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			printCertDetails(&out, certs, tt.now)

			assert.Contains(t, out.String(), "Subject:      CN=cd.example.com\n")
			assert.Contains(t, out.String(), "Issuer:       CN=cd.example.com\n")
			assert.Contains(t, out.String(), "Valid until:  "+x509Cert.NotAfter.UTC().Format(time.RFC3339)+tt.validity)
			assert.Contains(t, out.String(), "Fingerprint:  SHA256:"+certFingerprintSHA256(x509Cert)+"\n")
			assert.Contains(t, out.String(), "-----BEGIN CERTIFICATE-----")
		})
	}
}

// Data that cannot be decoded must not prevent the entry from being shown.
func TestPrintCertDetails_InvalidTLSData(t *testing.T) {
	var out bytes.Buffer
	printCertDetails(&out, []appsv1.RepositoryCertificate{{
		ServerName: "cd.example.com",
		CertType:   "https",
		CertData:   []byte("invalid"),
		CertInfo:   "could not decode PEM data from input",
	}}, time.Now())

	assert.Contains(t, out.String(), "Info:         could not decode PEM data from input\n")
	assert.Contains(t, out.String(), "Data:\ninvalid\n")
}
