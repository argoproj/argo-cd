package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/localconfig"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_applyCertConfig(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.crt")
	keyPath := filepath.Join(dir, "client.key")

	t.Run("persists the client certificate given on the command line", func(t *testing.T) {
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.NoError(t, applyCertConfig(&serverCfg, nil, certPath, keyPath))
		assert.Equal(t, certPath, serverCfg.ClientCertificate)
		assert.Equal(t, keyPath, serverCfg.ClientCertificateKey)
	})

	t.Run("stores the client certificate as an absolute path", func(t *testing.T) {
		t.Chdir(dir)
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.NoError(t, applyCertConfig(&serverCfg, nil, "client.crt", "client.key"))
		assert.Equal(t, certPath, serverCfg.ClientCertificate)
		assert.Equal(t, keyPath, serverCfg.ClientCertificateKey)
	})

	t.Run("carries over the certificate of a previous login", func(t *testing.T) {
		existing := &localconfig.Server{
			Server:                     "argocd.example.com",
			CACertificateAuthorityData: "ca-data",
			ClientCertificate:          certPath,
			ClientCertificateKey:       keyPath,
		}
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.NoError(t, applyCertConfig(&serverCfg, existing, "", ""))
		assert.Equal(t, "ca-data", serverCfg.CACertificateAuthorityData)
		assert.Equal(t, certPath, serverCfg.ClientCertificate)
		assert.Equal(t, keyPath, serverCfg.ClientCertificateKey)
	})

	t.Run("carries over inlined certificate data of a previous login", func(t *testing.T) {
		existing := &localconfig.Server{
			Server:                   "argocd.example.com",
			ClientCertificateData:    "Y2VydA==",
			ClientCertificateKeyData: "a2V5",
		}
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.NoError(t, applyCertConfig(&serverCfg, existing, "", ""))
		assert.Equal(t, "Y2VydA==", serverCfg.ClientCertificateData)
		assert.Equal(t, "a2V5", serverCfg.ClientCertificateKeyData)
	})

	t.Run("command line certificate replaces inlined certificate data", func(t *testing.T) {
		existing := &localconfig.Server{
			Server:                   "argocd.example.com",
			ClientCertificateData:    "Y2VydA==",
			ClientCertificateKeyData: "a2V5",
		}
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.NoError(t, applyCertConfig(&serverCfg, existing, certPath, keyPath))
		assert.Equal(t, certPath, serverCfg.ClientCertificate)
		assert.Equal(t, keyPath, serverCfg.ClientCertificateKey)
		assert.Empty(t, serverCfg.ClientCertificateData)
		assert.Empty(t, serverCfg.ClientCertificateKeyData)
	})

	t.Run("certificate and key must be given together", func(t *testing.T) {
		serverCfg := localconfig.Server{Server: "argocd.example.com"}
		require.ErrorContains(t, applyCertConfig(&serverCfg, nil, certPath, ""), "must always be specified together")
		require.ErrorContains(t, applyCertConfig(&serverCfg, nil, "", keyPath), "must always be specified together")
	})
}

func Test_buildVerificationPrompt_completeURI(t *testing.T) {
	prompt := buildVerificationPrompt("https://example.com/device?user_code=ABCD-1234", "https://example.com/device", "ABCD-1234")
	assert.Equal(t, "  https://example.com/device?user_code=ABCD-1234", prompt)
}

func Test_buildVerificationPrompt_noCompleteURI_plainBase(t *testing.T) {
	// Base URI has no query string — user_code is appended as the first parameter.
	prompt := buildVerificationPrompt("", "https://example.com/device", "ABCD-1234")
	assert.Contains(t, prompt, "https://example.com/device?user_code=ABCD-1234")
	assert.Contains(t, prompt, "ABCD-1234") // also shown for manual entry
}

func Test_buildVerificationPrompt_noCompleteURI_baseHasQuery(t *testing.T) {
	// Base URI already has a query parameter — user_code must be appended with & not ?.
	prompt := buildVerificationPrompt("", "https://example.com/device?foo=bar", "ABCD-1234")
	assert.Contains(t, prompt, "foo=bar")
	assert.Contains(t, prompt, "user_code=ABCD-1234")
	assert.NotContains(t, prompt, "?user_code") // must not add a second ?
}

func Test_buildVerificationPrompt_noCompleteURI_userCodeEncoded(t *testing.T) {
	// user_code with characters that need URL encoding.
	prompt := buildVerificationPrompt("", "https://example.com/device", "AB CD+12")
	assert.Contains(t, prompt, "user_code=AB+CD%2B12")
}

func Test_buildVerificationPrompt_invalidBaseURI(t *testing.T) {
	// Unparseable base URI falls back to plain text.
	prompt := buildVerificationPrompt("", "://bad url", "XXXX-YYYY")
	assert.Contains(t, prompt, "://bad url")
	assert.Contains(t, prompt, "XXXX-YYYY")
	assert.NotContains(t, prompt, "user_code=") // no URL synthesis attempted
}
