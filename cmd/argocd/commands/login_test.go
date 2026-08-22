package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	oidcutil "github.com/argoproj/argo-cd/v3/util/oidc"

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

// newDeviceCodeServer starts an httptest.Server that serves a fixed device code response.
func newDeviceCodeServer(t *testing.T, statusCode int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func Test_requestDeviceCode_success(t *testing.T) {
	want := oidcutil.OIDCDeviceCodeResponseBody{
		DeviceCode:              "dev-code-123",
		UserCode:                "ABCD-1234",
		VerificationURI:         "https://example.com/device",
		VerificationURIComplete: "https://example.com/device?user_code=ABCD-1234",
		ExpiresIn:               300,
		Interval:                5,
	}
	srv := newDeviceCodeServer(t, http.StatusOK, want)
	defer srv.Close()

	got, err := requestDeviceCode(context.Background(), srv.Client(), srv.URL, "my-client", "openid profile")

	require.NoError(t, err)
	assert.Equal(t, want, *got)
}

func Test_requestDeviceCode_wrongDeviceURL(t *testing.T) {
	// Unreachable URL — connection refused.
	_, err := requestDeviceCode(context.Background(), &http.Client{}, "http://127.0.0.1:1", "my-client", "openid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request device code")
}

func Test_requestDeviceCode_serverError(t *testing.T) {
	srv := newDeviceCodeServer(t, http.StatusInternalServerError, map[string]string{"error": "server_error"})
	defer srv.Close()

	_, err := requestDeviceCode(context.Background(), srv.Client(), srv.URL, "my-client", "openid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "device code request failed")
	assert.Contains(t, err.Error(), "500")
}

func Test_requestDeviceCode_malformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := requestDeviceCode(context.Background(), srv.Client(), srv.URL, "my-client", "openid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode device code response")
}

func Test_requestDeviceCode_wrongClientID(t *testing.T) {
	// Server echoes back the client_id it received; test asserts the field is forwarded.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		resp := oidcutil.OIDCDeviceCodeResponseBody{
			DeviceCode:      r.FormValue("client_id"), // echo client_id as DeviceCode for inspection
			UserCode:        "XXXX-XXXX",
			VerificationURI: "https://example.com/device",
			ExpiresIn:       300,
			Interval:        5,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// If the wrong client_id is sent the server would normally reject it;
	// here we verify the value is actually forwarded in the request.
	got, err := requestDeviceCode(context.Background(), srv.Client(), srv.URL, "wrong-client", "openid")

	require.NoError(t, err)
	assert.Equal(t, "wrong-client", got.DeviceCode, "client_id should be forwarded as-is")
}

func Test_requestDeviceCode_emptyScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		resp := oidcutil.OIDCDeviceCodeResponseBody{
			DeviceCode:      "dev-code",
			UserCode:        r.FormValue("scope"), // echo scope for inspection
			VerificationURI: "https://example.com/device",
			ExpiresIn:       300,
			Interval:        5,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	got, err := requestDeviceCode(context.Background(), srv.Client(), srv.URL, "my-client", "")

	require.NoError(t, err)
	assert.Empty(t, got.UserCode, "empty scope should be forwarded as empty string")
}

func Test_requestDeviceCode_cancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the request is made

	_, err := requestDeviceCode(ctx, &http.Client{}, "http://127.0.0.1:1", "my-client", "openid")

	require.Error(t, err)
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

// newTokenServer creates an httptest.Server that cycles through the provided
// responses in order, repeating the last one indefinitely.
func newTokenServer(t *testing.T, responses []tokenServerResponse) *httptest.Server {
	t.Helper()
	idx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		cur := responses[idx]
		if idx < len(responses)-1 {
			idx++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cur.status)
		_ = json.NewEncoder(w).Encode(cur.body)
	}))
}

type tokenServerResponse struct {
	status int
	body   any
}

func tokenOK(idToken, refreshToken string) tokenServerResponse {
	return tokenServerResponse{
		status: http.StatusOK,
		body:   map[string]string{"id_token": idToken, "refresh_token": refreshToken},
	}
}

func tokenErr(code string) tokenServerResponse {
	return tokenServerResponse{
		status: http.StatusBadRequest,
		body:   map[string]string{"error": code},
	}
}

func Test_pollForToken_success(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{tokenOK("id-tok", "ref-tok")})
	defer srv.Close()

	idToken, refreshToken, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.NoError(t, err)
	assert.Equal(t, "id-tok", idToken)
	assert.Equal(t, "ref-tok", refreshToken)
}

func Test_pollForToken_authorizationPending_thenSuccess(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{
		tokenErr("authorization_pending"),
		tokenErr("authorization_pending"),
		tokenOK("id-tok", "ref-tok"),
	})
	defer srv.Close()

	idToken, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.NoError(t, err)
	assert.Equal(t, "id-tok", idToken)
}

func Test_pollForToken_slowDown_thenSuccess(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{
		tokenErr("slow_down"),
		tokenOK("id-tok", ""),
	})
	defer srv.Close()

	idToken, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.NoError(t, err)
	assert.Equal(t, "id-tok", idToken)
}

func Test_pollForToken_expiredToken(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{tokenErr("expired_token")})
	defer srv.Close()

	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func Test_pollForToken_accessDenied(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{tokenErr("access_denied")})
	defer srv.Close()

	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func Test_pollForToken_unknownErrorCode(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{{status: http.StatusBadRequest, body: map[string]string{"error": "something_unexpected"}}})
	defer srv.Close()

	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "token request failed")
}

func Test_pollForToken_deadlineExceeded(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{tokenErr("authorization_pending")})
	defer srv.Close()

	// Deadline already in the past — first check exits immediately.
	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(-time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func Test_pollForToken_contextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := newTokenServer(t, []tokenServerResponse{tokenErr("authorization_pending")})
	defer srv.Close()

	// interval=5s so time.NewTimer(5s) can't fire before ctx.Done() on a pre-cancelled context.
	_, _, err := pollForToken(ctx, srv.Client(), srv.URL, "client", "dev-code", 5*time.Second, time.Now().Add(5*time.Minute))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancel")
}

func Test_pollForToken_noIDToken(t *testing.T) {
	srv := newTokenServer(t, []tokenServerResponse{
		{status: http.StatusOK, body: map[string]string{"access_token": "acc", "refresh_token": "ref"}},
	})
	defer srv.Close()

	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "id_token")
}

func Test_pollForToken_malformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, _, err := pollForToken(context.Background(), srv.Client(), srv.URL, "client", "dev-code", time.Millisecond, time.Now().Add(30*time.Second))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode token response")
}
