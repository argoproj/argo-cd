package commands

import (
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"testing"

	utilio "github.com/argoproj/argo-cd/v3/util/io"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func captureStdout(callback func()) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	callback()
	utilio.Close(w)

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func Test_userDisplayName_email(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "firstname.lastname@example.com", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "firstname.lastname@example.com"
	assert.Equal(t, expectedName, actualName)
}

func Test_userDisplayName_name(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "name": "Firstname Lastname", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "Firstname Lastname"
	assert.Equal(t, expectedName, actualName)
}

func Test_userDisplayName_sub(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "foo"
	assert.Equal(t, expectedName, actualName)
}

func Test_userDisplayName_federatedClaims(t *testing.T) {
	claims := jwt.MapClaims{
		"iss":    "qux",
		"sub":    "foo",
		"groups": []string{"baz"},
		"federated_claims": map[string]any{
			"connector_id": "dex",
			"user_id":      "ldap-123",
		},
	}
	actualName := userDisplayName(claims)
	expectedName := "ldap-123"
	assert.Equal(t, expectedName, actualName)
}

func Test_ssoAuthFlow_ssoLaunchBrowser_false(t *testing.T) {
	out, _ := captureStdout(func() {
		ssoAuthFlow("http://test-sso-browser-flow.com", false)
	})

	assert.Contains(t, out, "To authenticate, copy-and-paste the following URL into your preferred browser: http://test-sso-browser-flow.com")
}

func Test_generateSelfSignedCert(t *testing.T) {
	host := "localhost"
	certFile, keyFile, err := generateSelfSignedCert(host)
	assert.NoError(t, err)

	// Check if the certificate file is created
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		t.Errorf("Expected certificate file %s to exist, but it does not", certFile)
	}

	// Check if the key file is created
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Errorf("Expected key file %s to exist, but it does not", keyFile)
	}

	// Read and parse the certificate
	certData, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("Failed to read certificate file %s: %v", certFile, err)
	}

	certPem, _ := pem.Decode(certData)

	cert, err := x509.ParseCertificate(certPem.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate from %s: %s", certFile, err)
	}

	// Validate the host in the certificate matches the requested host
	if cert.Subject.CommonName != host {
		t.Errorf("Expected certificate subject common name to be %s, but got %s", host, cert.Subject.CommonName)
	}
}
