package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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

// TestBrowserHelperProcess is not a real test: Test_launchBrowser runs the test binary itself as the
// browser, which lands here and records its arguments after "--". This works on every OS, Windows included.
func TestBrowserHelperProcess(_ *testing.T) {
	out := os.Getenv("BROWSER_TEST_OUT")
	if out == "" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	_ = os.WriteFile(out, []byte(strings.Join(args, " ")), 0o600)
	os.Exit(0)
}

func Test_launchBrowser(t *testing.T) {
	const url = "http://test-sso-browser-flow.com/auth?a=1&b=2"
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	exe, err := os.Executable()
	require.NoError(t, err)
	browser := `"` + exe + `" -test.run=^TestBrowserHelperProcess$ --`
	t.Setenv("BROWSER_TEST_OUT", out)

	invoked := func(t *testing.T) string {
		t.Helper()
		var got []byte
		require.Eventually(t, func() bool {
			var err error
			got, err = os.ReadFile(out)
			return err == nil && len(got) > 0
		}, 5*time.Second, 10*time.Millisecond)
		require.NoError(t, os.Remove(out))
		return strings.TrimSpace(string(got))
	}

	t.Run("appends the URL to the command", func(t *testing.T) {
		t.Setenv("BROWSER", browser+" --new-window")
		require.NoError(t, launchBrowser(url))
		assert.Equal(t, "--new-window "+url, invoked(t))
	})

	t.Run("replaces an argument that is exactly %s with the URL", func(t *testing.T) {
		t.Setenv("BROWSER", browser+` --profile-directory="Profile 1" %s --new-window`)
		require.NoError(t, launchBrowser(url))
		assert.Equal(t, "--profile-directory=Profile 1 "+url+" --new-window", invoked(t))
	})

	t.Run("never splices the URL into a larger argument", func(t *testing.T) {
		t.Setenv("BROWSER", browser+` "open %s"`)
		require.NoError(t, launchBrowser(url))
		assert.Equal(t, "open %s "+url, invoked(t))
	})

	t.Run("falls back to the next command when one cannot start", func(t *testing.T) {
		t.Setenv("BROWSER", filepath.Join(dir, "missing")+string(os.PathListSeparator)+browser)
		require.NoError(t, launchBrowser(url))
		assert.Equal(t, url, invoked(t))
	})

	t.Run("refuses a URL that is not absolute http(s)", func(t *testing.T) {
		t.Setenv("BROWSER", browser)
		for _, bad := range []string{"--new-window", "-https://example.com", "file:///etc/passwd", "javascript:alert(1)", "https://"} {
			require.ErrorContains(t, launchBrowser(bad), "not an absolute http(s) URL", bad)
		}
		_, err := os.Stat(out)
		assert.True(t, os.IsNotExist(err), "browser must not be started")
	})

	t.Run("fails when no command can start", func(t *testing.T) {
		t.Setenv("BROWSER", filepath.Join(dir, "missing")+string(os.PathListSeparator))
		err := launchBrowser(url)
		require.ErrorContains(t, err, "no usable browser in BROWSER")
		require.ErrorContains(t, err, "empty BROWSER entry")
	})
}

func Test_parseBrowserEnv(t *testing.T) {
	// parse returns each entry's arguments, and the indexes of the entries that failed to parse.
	parse := func(browsers string, windows bool) ([][]string, []int) {
		var args [][]string
		var failed []int
		for i, e := range parseBrowserEnv(browsers, windows) {
			args = append(args, e.args)
			if e.err != nil {
				failed = append(failed, i)
			}
		}
		return args, failed
	}

	t.Run("windows keeps a quoted executable path and its backslashes", func(t *testing.T) {
		args, failed := parse(`"C:\Program Files\Google\Chrome\Application\chrome.exe" --profile-directory="Profile 1" %s`, true)
		assert.Equal(t, [][]string{{`C:\Program Files\Google\Chrome\Application\chrome.exe`, "--profile-directory=Profile 1", "%s"}}, args)
		assert.Empty(t, failed)
	})

	t.Run("windows splits commands on ; outside quotes only", func(t *testing.T) {
		args, failed := parse(`C:\missing.exe;"C:\a;b\browser.exe" --new-window;`, true)
		assert.Equal(t, [][]string{{`C:\missing.exe`}, {`C:\a;b\browser.exe`, "--new-window"}, nil}, args)
		assert.Empty(t, failed)
	})

	t.Run("windows treats a backslash-escaped quote as literal", func(t *testing.T) {
		args, failed := parse(`"C:\a\"b;c" --x;"C:\dir\\" --y`, true)
		assert.Equal(t, [][]string{{`C:\a"b;c`, "--x"}, {`C:\dir\`, "--y"}}, args)
		assert.Empty(t, failed)
	})

	t.Run("windows reports an unterminated quote", func(t *testing.T) {
		_, failed := parse(`"C:\Program Files\browser.exe`, true)
		assert.Equal(t, []int{0}, failed)
	})

	t.Run("posix splits commands on : and follows shell quoting", func(t *testing.T) {
		args, failed := parse(`/missing:open -na "Google Chrome" --args --profile-directory="Profile 1" %s`, false)
		assert.Equal(t, [][]string{{"/missing"}, {"open", "-na", "Google Chrome", "--args", "--profile-directory=Profile 1", "%s"}}, args)
		assert.Empty(t, failed)
	})

	t.Run("posix splits commands on : outside quotes only", func(t *testing.T) {
		args, failed := parse(`firefox -P "Work:Cloud" %s:chromium --user-data-dir='/tmp/a:b' %s:/opt/x\:y/run`, false)
		assert.Equal(t, [][]string{{"firefox", "-P", "Work:Cloud", "%s"}, {"chromium", "--user-data-dir=/tmp/a:b", "%s"}, {"/opt/x:y/run"}}, args)
		assert.Empty(t, failed)
	})

	t.Run("posix reports an unterminated quote", func(t *testing.T) {
		_, failed := parse(`firefox -P "Work`, false)
		assert.Equal(t, []int{0}, failed)
	})

	t.Run("posix keeps backslash escapes", func(t *testing.T) {
		args, failed := parse(`/opt/my\ browser/run %s`, false)
		assert.Equal(t, [][]string{{"/opt/my browser/run", "%s"}}, args)
		assert.Empty(t, failed)
	})
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
