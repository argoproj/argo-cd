package logout

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	httputil "github.com/argoproj/argo-cd/v3/util/http"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/session"
)

// NewHandler creates handler serving to do api/logout endpoint
func NewHandler(sessionMgr *session.SessionManager, configProvider configbus.Provider) *Handler {
	return &Handler{
		configProvider: configProvider,
		verifyToken:    sessionMgr.VerifyToken,
		revokeToken:    sessionMgr.RevokeToken,
	}
}

type Handler struct {
	configProvider configbus.Provider
	verifyToken    func(ctx context.Context, tokenString string) (jwt.Claims, string, error)
	revokeToken    func(ctx context.Context, id string, expiringAt time.Duration) error
}

var (
	tokenPattern             = regexp.MustCompile(`{{token}}`)
	logoutRedirectURLPattern = regexp.MustCompile(`{{logoutRedirectURL}}`)
)

func constructLogoutURL(logoutURL, token, logoutRedirectURL string) string {
	constructedLogoutURL := tokenPattern.ReplaceAllString(logoutURL, token)
	return logoutRedirectURLPattern.ReplaceAllString(constructedLogoutURL, logoutRedirectURL)
}

func argoURLForRequest(r *http.Request, serverURL string, additionalURLs []string) (string, error) {
	for _, candidateURL := range append([]string{serverURL}, additionalURLs...) {
		u, err := url.Parse(candidateURL)
		if err != nil {
			return "", err
		}
		if u.Host == r.Host && strings.HasPrefix(r.URL.RequestURI(), u.RequestURI()) {
			return candidateURL, nil
		}
	}
	return serverURL, nil
}

// ServeHTTP is the logout handler for ArgoCD and constructs OIDC logout URL and redirects to it for OIDC issued sessions,
// and redirects user to '/login' for argocd issued sessions
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rootPath, err := h.configProvider.RootPath(r.Context())
	if err != nil {
		http.Error(w, "Failed to resolve root path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	baseHRef, err := h.configProvider.BaseHRef(r.Context())
	if err != nil {
		http.Error(w, "Failed to resolve base href: "+err.Error(), http.StatusInternalServerError)
		return
	}
	serverURL, err := h.configProvider.ServerURL(r.Context())
	if err != nil {
		http.Error(w, "Failed to resolve server URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	additionalURLs, err := h.configProvider.AdditionalURLs(r.Context())
	if err != nil {
		http.Error(w, "Failed to resolve additional URLs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	argoURL, err := argoURLForRequest(r, serverURL, additionalURLs)
	if err != nil {
		log.Warnf("unable to find ArgoCD URL from config: %v", err)
	}
	if argoURL == "" {
		// golang does not provide any easy way to determine scheme of current request
		// so redirecting ot http which will auto-redirect too https if necessary
		host := strings.TrimRight(r.Host, "/")
		argoURL = "http://" + host + "/" + strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/")
	}

	logoutRedirectURL := strings.TrimRight(strings.TrimLeft(argoURL, "/"), "/")

	cookies := r.Cookies()
	tokenString, err := httputil.JoinCookies(common.AuthCookieName, cookies)
	// Build message safely: only include err when non-nil
	if err != nil {
		http.Error(w, "Failed to retrieve ArgoCD auth token: "+err.Error(), http.StatusBadRequest)
		return
	}
	if tokenString == "" {
		http.Error(w, "Failed to retrieve ArgoCD auth token", http.StatusBadRequest)
		return
	}

	for _, cookie := range cookies {
		if !strings.HasPrefix(cookie.Name, common.AuthCookieName) {
			continue
		}

		argocdCookie := http.Cookie{
			Name:  cookie.Name,
			Value: "",
		}

		argocdCookie.Path = "/" + strings.TrimRight(strings.TrimLeft(baseHRef, "/"), "/")
		w.Header().Add("Set-Cookie", argocdCookie.String())
	}

	claims, _, err := h.verifyToken(r.Context(), tokenString)
	if err != nil {
		http.Redirect(w, r, logoutRedirectURL, http.StatusSeeOther)
		return
	}

	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		http.Redirect(w, r, logoutRedirectURL, http.StatusSeeOther)
		return
	}

	issuer := jwtutil.StringField(mapClaims, "iss")
	id := jwtutil.StringField(mapClaims, "jti")
	// Workaround for Dex token, because does not have jti.
	if id == "" {
		id = jwtutil.StringField(mapClaims, "at_hash")
	}

	if exp, err := jwtutil.ExpirationTime(mapClaims); err == nil && id != "" {
		ttl := time.Until(exp)
		if ttl <= 0 {
			// Token already expired; no need to persist revocation
			log.Infof("token '%s' already expired, skipping revocation", id)
		} else {
			revokeCtx, cancel := context.WithTimeout(r.Context(), common.TokenRevocationTimeout)
			defer cancel()
			if err := h.revokeToken(revokeCtx, id, ttl); err != nil {
				log.Warnf("failed to invalidate token '%s': %v", id, err)
			}
		}
	}

	oidcLogoutURL, err := h.configProvider.OIDCLogoutURL(r.Context())
	if err != nil {
		http.Error(w, "Failed to resolve OIDC logout URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if oidcLogoutURL == "" || issuer == session.SessionManagerClaimsIssuer {
		http.Redirect(w, r, logoutRedirectURL, http.StatusSeeOther)
	} else {
		logoutURL := constructLogoutURL(oidcLogoutURL, tokenString, logoutRedirectURL)
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
	}
}
