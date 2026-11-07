package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/localconfig"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
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

func Test_buildVerificationPrompt_noCompleteURI(t *testing.T) {
	// RFC 8628 §3.3: when verification_uri_complete is absent, show the base
	// URI and the user_code separately. No URL synthesis is performed.
	prompt := buildVerificationPrompt("", "https://example.com/device", "ABCD-1234")
	assert.Equal(t, "  https://example.com/device\n\n  Enter the code: ABCD-1234", prompt)
}

func Test_buildVerificationPrompt_invalidBaseURI(t *testing.T) {
	// Even an unparseable URI is shown as-is alongside the code.
	prompt := buildVerificationPrompt("", "://bad url", "XXXX-YYYY")
	assert.Equal(t, "  ://bad url\n\n  Enter the code: XXXX-YYYY", prompt)
}

// pkceCharsetRe matches the unreserved characters allowed by RFC 7636 §4.1.
var pkceCharsetRe = regexp.MustCompile(`^[A-Za-z0-9\-._~]+$`)

func Test_generatePKCECodeVerifierAndChallenge_noError(t *testing.T) {
	verifier, challenge, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)
	assert.NotEmpty(t, verifier)
	assert.NotEmpty(t, challenge)
}

func Test_generatePKCECodeVerifierAndChallenge_verifierLength(t *testing.T) {
	// RFC 7636 §4.1 mandates a code_verifier of 43–128 characters; the
	// implementation always produces exactly 43.
	verifier, _, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)
	assert.Len(t, verifier, 43)
}

func Test_generatePKCECodeVerifierAndChallenge_verifierCharset(t *testing.T) {
	// RFC 7636 §4.1: allowed characters are ALPHA / DIGIT / "-" / "." / "_" / "~"
	verifier, _, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)
	assert.Regexp(t, pkceCharsetRe, verifier, "verifier must only contain RFC 7636 unreserved characters")
}

func Test_generatePKCECodeVerifierAndChallenge_challengeIsS256OfVerifier(t *testing.T) {
	// RFC 7636 §4.2: code_challenge = BASE64URL(SHA256(ASCII(code_verifier)))
	verifier, challenge, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)

	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	assert.Equal(t, expected, challenge, "challenge must be BASE64URL(SHA256(verifier))")
}

func Test_generatePKCECodeVerifierAndChallenge_challengeHasNoPadding(t *testing.T) {
	// BASE64URL encoding without padding must not contain '=' characters.
	_, challenge, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)
	assert.NotContains(t, challenge, "=", "challenge must use unpadded base64url encoding")
}

func Test_generatePKCECodeVerifierAndChallenge_challengeIsBase64URL(t *testing.T) {
	// The challenge must contain only base64url alphabet characters (A-Z, a-z, 0-9, '-', '_').
	_, challenge, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)
	assert.Regexp(t, `^[A-Za-z0-9_-]+$`, challenge, "challenge must be a valid base64url string")
}

func Test_generatePKCECodeVerifierAndChallenge_uniqueness(t *testing.T) {
	// Two successive calls must not produce the same verifier.  The probability
	// of a collision is negligibly small (~1/52^43).
	v1, c1, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)

	v2, c2, err := generatePKCECodeVerifierAndChallenge()
	require.NoError(t, err)

	assert.NotEqual(t, v1, v2, "successive verifiers should be unique")
	assert.NotEqual(t, c1, c2, "successive challenges should be unique")
}

type coreLoginConfig struct {
	CurrentContext string `json:"current-context"`
	Contexts       []struct {
		Name       string `json:"name"`
		KubeConfig string `json:"kubeconfig"`
	} `json:"contexts"`
}

// coreLogin executes a core-mode login and reads the resulting CLI configuration.
func coreLogin(t *testing.T, configPath string, args ...string) coreLoginConfig {
	t.Helper()
	cmd := NewLoginCommand(&apiclient.ClientOptions{Core: true, ConfigPath: configPath})
	cmd.SetArgs(args)
	var commandErr error
	_, err := captureStdout(func() {
		commandErr = cmd.ExecuteContext(t.Context())
	})
	require.NoError(t, err)
	require.NoError(t, commandErr)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var cfg coreLoginConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	return cfg
}

// TestLoginCoreKubeConfigPath checks absolute path persistence without copying kubeconfig credentials.
func TestLoginCoreKubeConfigPath(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute"
		if relative {
			name = "relative"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			path := filepath.Join(dir, "k3s.yaml")
			require.NoError(t, os.WriteFile(path, []byte("token: do-not-copy-this-token\n"), 0o600))
			flagPath := path
			if relative {
				flagPath = "k3s.yaml"
			}
			configPath := filepath.Join(dir, "argocd-config")
			cfg := coreLogin(t, configPath, "--name", "k3s", "--kubeconfig", flagPath)
			require.Len(t, cfg.Contexts, 1)
			assert.Equal(t, "k3s", cfg.CurrentContext)
			assert.Equal(t, path, cfg.Contexts[0].KubeConfig)
			data, err := os.ReadFile(configPath)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "do-not-copy-this-token")
		})
	}
}

// TestLoginCoreKubeConfigContexts verifies that named core contexts retain independent kubeconfig paths.
func TestLoginCoreKubeConfigContexts(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "argocd-config")
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	require.NoError(t, os.WriteFile(first, nil, 0o600))
	require.NoError(t, os.WriteFile(second, nil, 0o600))
	coreLogin(t, configPath, "--name", "first", "--kubeconfig", first)
	cfg := coreLogin(t, configPath, "--name", "second", "--kubeconfig", second)
	require.Len(t, cfg.Contexts, 2)
	assert.Equal(t, "second", cfg.CurrentContext)
	assert.Equal(t, first, cfg.Contexts[0].KubeConfig)
	assert.Equal(t, second, cfg.Contexts[1].KubeConfig)
}

// TestLoginCoreWithoutKubeConfig verifies that omitting the flag clears a previously saved path.
func TestLoginCoreWithoutKubeConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "argocd-config")
	t.Setenv("KUBECONFIG", filepath.Join(dir, "environment.yaml"))
	saved := filepath.Join(dir, "saved.yaml")
	require.NoError(t, os.WriteFile(saved, nil, 0o600))
	coreLogin(t, configPath, "--name", "core", "--kubeconfig", saved)
	cfg := coreLogin(t, configPath, "--name", "core")
	require.Len(t, cfg.Contexts, 1)
	assert.Empty(t, cfg.Contexts[0].KubeConfig)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "kubeconfig:")
}

// TestLoginKubeConfigValidation rejects unusable explicit paths and flags outside core mode before writing configuration.
func TestLoginKubeConfigValidation(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "kubeconfig")
	require.NoError(t, os.WriteFile(existing, nil, 0o600))
	for _, tc := range []struct {
		name      string
		core      bool
		path      string
		wantError string
	}{
		{name: "missing file", core: true, path: filepath.Join(dir, "missing"), wantError: "cannot access kubeconfig"},
		{name: "directory", core: true, path: dir, wantError: "is a directory"},
		{name: "flag without core mode", path: existing, wantError: "--kubeconfig requires --core"},
		{name: "empty flag without core mode", wantError: "--kubeconfig requires --core"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "argocd-config")
			before := []byte("existing configuration must remain unchanged\n")
			require.NoError(t, os.WriteFile(configPath, before, 0o600))
			cmd := NewLoginCommand(&apiclient.ClientOptions{Core: tc.core, ConfigPath: configPath})
			cmd.SetArgs([]string{"--kubeconfig", tc.path})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			require.ErrorContains(t, cmd.ExecuteContext(t.Context()), tc.wantError)
			after, err := os.ReadFile(configPath)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
	t.Run("ordinary login does not require kubeconfig", func(t *testing.T) {
		cmd := NewLoginCommand(&apiclient.ClientOptions{})
		require.NoError(t, cmd.PreRunE(cmd, nil))
	})
}
