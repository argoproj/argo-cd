package logout

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/dgrijalva/jwt-go"
)

//NewHandler creates handler serving to do api/logout endpoint

func NewHandler(appClientset versioned.Interface, settingsMrg *settings.SettingsManager, sessionMgr *session.SessionManager, rootPath, namespace string) *Handler {
	return &Handler{
		appClientset: appClientset,
		namespace:    namespace,
		settingsMgr:  settingsMrg,
		rootPath:     rootPath,
		verifyToken:  sessionMgr.VerifyToken,
	}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
	rootPath     string
	verifyToken  func(tokenString string) (jwt.Claims, error)
}

var (
	tokenPattern             = regexp.MustCompile(`{{token}}`)
	logoutRedirectURLPattern = regexp.MustCompile(`{{logoutRedirectURL}}`)
)

func constructLogoutURL(logoutURL, token, logoutRedirectURL string) string {
	constructedLogoutURL := tokenPattern.ReplaceAllString(logoutURL, token)
	return logoutRedirectURLPattern.ReplaceAllString(constructedLogoutURL, logoutRedirectURL)
}

// ServeHTTP is the logout handler for ArgoCD and constructs OIDC logout URL and redirects to it for OIDC issued sessions,
// and redirects user to '/login' for argocd issued sessions
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var tokenString string
	var OIDCConfig *settings.OIDCConfig

	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to retrieve argoCD settings: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}

	argocdCookie, err := r.Cookie(common.AuthCookieName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		http.Error(w, "Failed to retrieve ArgoCD auth token: "+fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}

	tokenString = argocdCookie.Value

	claims, err := h.verifyToken(tokenString)
	if err != nil {
		cookie := fmt.Sprintf("%s=", common.AuthCookieName)
		w.Header().Set("Set-Cookie", cookie)
		redirectURL := argoCDSettings.URL + "/login"
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}

	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		cookie := fmt.Sprintf("%s=", common.AuthCookieName)
		w.Header().Set("Set-Cookie", cookie)
		redirectURL := argoCDSettings.URL + "/login"
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}

	issuer := jwtutil.GetField(mapClaims, "iss")

	cookie := fmt.Sprintf("%s=", common.AuthCookieName)
	w.Header().Set("Set-Cookie", cookie)

	if argoCDSettings.OIDCConfig() == nil || issuer == session.SessionManagerClaimsIssuer {
		redirectURL := argoCDSettings.URL + "/login"
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	} else {
		OIDCConfig = argoCDSettings.OIDCConfig()
		LogoutRedirectURL := argoCDSettings.URL + strings.TrimRight(strings.TrimLeft(h.rootPath, "/"), "/")
		logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString, LogoutRedirectURL)
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
	}
}
