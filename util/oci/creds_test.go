package oci

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCredentialFunc_Static(t *testing.T) {
	f := NewCredentialFunc("registry.example.com", Creds{Username: "user", Password: "pass"})

	cred, err := f(context.Background(), "registry.example.com")
	require.NoError(t, err)
	assert.Equal(t, "user", cred.Username)
	assert.Equal(t, "pass", cred.Password)

	// StaticCredential is scoped to the registry: a different host returns empty credentials.
	other, err := f(context.Background(), "other.example.com")
	require.NoError(t, err)
	assert.Empty(t, other.Username)
	assert.Empty(t, other.Password)
}

func TestNewCredentialFunc_GCP_InvalidKey(t *testing.T) {
	f := NewCredentialFunc("us-docker.pkg.dev", Creds{GCPServiceAccountKey: "not-json"})
	_, err := f(context.Background(), "us-docker.pkg.dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse GCP service account key")
}

func TestGCPTokenSource_Caches(t *testing.T) {
	jsonKey := generateServiceAccountKey(t)
	before := googleCloudTokenSource.ItemCount()

	_, err := gcpTokenSource(context.Background(), jsonKey)
	require.NoError(t, err)
	_, err = gcpTokenSource(context.Background(), jsonKey)
	require.NoError(t, err)

	// The second lookup must hit the cache, so only one entry is added for a given key.
	assert.Equal(t, before+1, googleCloudTokenSource.ItemCount())
}

// generateServiceAccountKey builds a syntactically valid GCP service account key JSON with a
// freshly generated RSA private key. It is sufficient to construct an oauth2.TokenSource without
// contacting Google (no token is actually minted).
func generateServiceAccountKey(t *testing.T) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	key, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"client_email":   "test@example.iam.gserviceaccount.com",
		"private_key":    string(pemBytes),
		"private_key_id": "test-key-id",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)
	return key
}
