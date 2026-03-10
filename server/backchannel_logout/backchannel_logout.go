// Package backchannellogout implements the OIDC Back-Channel Logout endpoint
// as specified in https://openid.net/specs/openid-connect-backchannel-1_0.html.
//
// When an external OIDC provider (or Dex) terminates a user session, it sends a
// signed Logout Token to this endpoint. ArgoCD verifies the token and immediately
// revokes the corresponding server-side session so that subsequent requests carrying
// a token with that session ID are rejected, even before their JWT expiry time.
package backchannellogout

import (
	"context"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	oidcutil "github.com/argoproj/argo-cd/v3/util/oidc"
	"github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// defaultRevocationTTL is the duration for which a revoked OIDC session entry is kept in Redis.
// It is set conservatively so that all plausible tokens that carry the revoked session ID will
// have expired before the entry is removed.
const defaultRevocationTTL = 24 * time.Hour

// Handler handles POST /auth/backchannel-logout requests from an OIDC provider.
type Handler struct {
	getSettings       func() (*settings.ArgoCDSettings, error)
	verifyLogoutToken func(ctx context.Context, tokenString string) (*oidcutil.LogoutTokenClaims, error)
	revokeOIDCSession func(ctx context.Context, sid string, expiration time.Duration) error
	invalidateCache   func(sub, sid string)
}

// NewHandler creates a new backchannel logout handler.
// The settingsMgr, sessionMgr, and clientApp parameters provide the concrete implementations
// used in production; individual function fields may be overridden in tests.
func NewHandler(settingsMgr *settings.SettingsManager, sessionMgr *session.SessionManager, clientApp *oidcutil.ClientApp) *Handler {
	return &Handler{
		getSettings:       settingsMgr.GetSettings,
		verifyLogoutToken: clientApp.VerifyLogoutToken,
		revokeOIDCSession: sessionMgr.RevokeOIDCSession,
		invalidateCache:   clientApp.InvalidateSessionCache,
	}
}

// ServeHTTP implements the OIDC Back-Channel Logout endpoint.
//
// The IdP MUST call this endpoint with a POST request carrying:
//   - Content-Type: application/x-www-form-urlencoded
//   - A form field named "logout_token" whose value is a signed Logout Token JWT.
//
// On success the handler returns HTTP 200. On any error it returns an appropriate
// 4xx/5xx status and logs the details server-side (without leaking internal state).
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// The spec requires application/x-www-form-urlencoded; be lenient about optional params.
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		http.Error(w, "Content-Type must be application/x-www-form-urlencoded", http.StatusUnsupportedMediaType)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Warnf("backchannel-logout: failed to parse request body: %v", err)
		http.Error(w, "failed to parse request body", http.StatusBadRequest)
		return
	}

	tokenString := r.FormValue("logout_token")
	if tokenString == "" {
		http.Error(w, "missing logout_token parameter", http.StatusBadRequest)
		return
	}

	argoCDSettings, err := h.getSettings()
	if err != nil {
		log.Errorf("backchannel-logout: failed to retrieve ArgoCD settings: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if argoCDSettings.OIDCConfig() == nil {
		http.Error(w, "OIDC is not configured", http.StatusNotImplemented)
		return
	}

	claims, err := h.verifyLogoutToken(r.Context(), tokenString)
	if err != nil {
		// Log with details internally but do not expose them to the caller.
		log.Warnf("backchannel-logout: failed to verify logout token: %v", err)
		http.Error(w, "invalid logout token", http.StatusBadRequest)
		return
	}

	// context.Background() is intentional: revocation must complete even if the IdP's
	// HTTP connection drops or times out before we respond.
	if err := h.revokeOIDCSession(context.Background(), claims.Sid, defaultRevocationTTL); err != nil {
		log.Errorf("backchannel-logout: failed to revoke OIDC session (sid=%s sub=%s): %v", claims.Sid, claims.Sub, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Best-effort cache invalidation; errors are logged inside InvalidateSessionCache.
	h.invalidateCache(claims.Sub, claims.Sid)

	log.Infof("backchannel-logout: revoked OIDC session (sid=%s sub=%s)", claims.Sid, claims.Sub)
	w.WriteHeader(http.StatusOK)
}
