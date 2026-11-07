package github_app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
)

func TestResolveAPIURLPrefersCredentialEnterpriseURL(t *testing.T) {
	enterpriseURL := "https://api.example.ghe.com"
	configuredURL := "https://api.other.example.com"

	assert.Equal(t, enterpriseURL, resolveAPIURL(github_app_auth.Authentication{
		EnterpriseBaseURL: enterpriseURL,
	}, configuredURL))
	assert.Equal(t, configuredURL, resolveAPIURL(github_app_auth.Authentication{}, configuredURL))
}

// Covers the call site in getInstallationClient, not just the helper: #28981 was
// that a credential's EnterpriseBaseURL never reached the client when a URL was
// also configured, so asserting the resolved string alone would not have caught
// it. ghinstallation only parses the key, so this stays offline.
func TestGetInstallationClientUsesCredentialEnterpriseURL(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	client, err := getInstallationClient(github_app_auth.Authentication{
		Id:                1,
		InstallationId:    2,
		PrivateKey:        string(pem),
		EnterpriseBaseURL: "https://api.example.ghe.com",
	}, "https://api.other.example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.ghe.com/", client.BaseURL.String())
}
