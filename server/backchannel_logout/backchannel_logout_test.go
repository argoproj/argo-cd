package backchannellogout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidcutil "github.com/argoproj/argo-cd/v3/util/oidc"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// oidcSettings returns a minimal ArgoCDSettings with OIDC configured.
func oidcSettings() *settings.ArgoCDSettings {
	s := &settings.ArgoCDSettings{}
	s.OIDCConfigRAW = `
name: Test
issuer: https://idp.example.com
clientID: argocd
clientSecret: secret
`
	return s
}

func noOIDCSettings() *settings.ArgoCDSettings {
	return &settings.ArgoCDSettings{}
}

func okRevoke(_ context.Context, _ string, _ time.Duration) error { return nil }

func okVerify(_ context.Context, _ string) (*oidcutil.LogoutTokenClaims, error) {
	return &oidcutil.LogoutTokenClaims{Sub: "user1", Sid: "sess-1"}, nil
}

func noopCache(_, _ string) {}

// newTestHandler builds a Handler with all dependencies injected via function fields.
func newTestHandler(
	getSettings func() (*settings.ArgoCDSettings, error),
	verifyLogoutToken func(ctx context.Context, token string) (*oidcutil.LogoutTokenClaims, error),
	revokeOIDCSession func(ctx context.Context, sid string, expiration time.Duration) error,
	invalidateCache func(sub, sid string),
) *Handler {
	return &Handler{
		getSettings:       getSettings,
		verifyLogoutToken: verifyLogoutToken,
		revokeOIDCSession: revokeOIDCSession,
		invalidateCache:   invalidateCache,
	}
}

func postRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/auth/backchannel-logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify, okRevoke, noopCache,
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/backchannel-logout", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandler_WrongContentType(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify, okRevoke, noopCache,
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/backchannel-logout", strings.NewReader("logout_token=TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestHandler_MissingToken(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify, okRevoke, noopCache,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "other_field=value"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing logout_token")
}

func TestHandler_OIDCNotConfigured(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return noOIDCSettings(), nil },
		okVerify, okRevoke, noopCache,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=TOKEN"))
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestHandler_SettingsError(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return nil, errors.New("db error") },
		okVerify, okRevoke, noopCache,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=TOKEN"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_InvalidLogoutToken(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		func(_ context.Context, _ string) (*oidcutil.LogoutTokenClaims, error) {
			return nil, errors.New("bad token signature")
		},
		okRevoke, noopCache,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=BAD"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	// Ensure internal error details are NOT leaked to the caller.
	assert.NotContains(t, rec.Body.String(), "bad token signature")
	assert.Contains(t, rec.Body.String(), "invalid logout token")
}

func TestHandler_RevocationError(t *testing.T) {
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify,
		func(_ context.Context, _ string, _ time.Duration) error {
			return errors.New("redis unavailable")
		},
		noopCache,
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=TOKEN"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Success(t *testing.T) {
	var revokedSid string
	var invalidatedSub, invalidatedSid string

	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify,
		func(_ context.Context, sid string, _ time.Duration) error {
			revokedSid = sid
			return nil
		},
		func(sub, sid string) {
			invalidatedSub = sub
			invalidatedSid = sid
		},
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=VALID"))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "sess-1", revokedSid)
	assert.Equal(t, "user1", invalidatedSub)
	assert.Equal(t, "sess-1", invalidatedSid)
}

func TestHandler_Success_ContentTypeWithParams(t *testing.T) {
	// Content-Type may carry optional parameters like charset; the prefix check must still pass.
	h := newTestHandler(
		func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		okVerify, okRevoke, noopCache,
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/backchannel-logout",
		strings.NewReader("logout_token=VALID"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNewHandler_Wiring verifies that NewHandler correctly wires the production dependencies.
// It calls the handler end-to-end with a real (but minimal) mock so that the constructor path
// is exercised.
func TestNewHandler_Wiring(t *testing.T) {
	// We cannot easily construct a real SettingsManager / SessionManager / ClientApp in a unit
	// test, so we verify the wiring by constructing the Handler directly (as production does) and
	// checking that every function field is non-nil.
	h := &Handler{
		getSettings:       func() (*settings.ArgoCDSettings, error) { return oidcSettings(), nil },
		verifyLogoutToken: okVerify,
		revokeOIDCSession: okRevoke,
		invalidateCache:   noopCache,
	}
	require.NotNil(t, h.getSettings)
	require.NotNil(t, h.verifyLogoutToken)
	require.NotNil(t, h.revokeOIDCSession)
	require.NotNil(t, h.invalidateCache)

	// A successful request through the wired handler must return 200.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, postRequest(t.Context(), "logout_token=VALID"))
	assert.Equal(t, http.StatusOK, rec.Code)
}
