package logout

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/dgrijalva/jwt-go/v4"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"

	jwtutil "github.com/argoproj/argo-cd/util/jwt"
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
	var oidcConfig *settings.OIDCConfig

	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to retrieve argoCD settings: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}

	logoutRedirectURL := strings.TrimRight(strings.TrimLeft(argoCDSettings.URL, "/"), "/") + strings.TrimRight(strings.TrimLeft(h.rootPath, "/"), "/")

	argocdCookie, err := r.Cookie(common.AuthCookieName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		http.Error(w, "Failed to retrieve ArgoCD auth token: "+fmt.Sprintf("%s", err), http.StatusBadRequest)
		return
	}

	tokenString = argocdCookie.Value

	argocdCookie.Value = ""
	argocdCookie.Path = fmt.Sprintf("/%s", strings.TrimRight(strings.TrimLeft(h.rootPath, "/"), "/"))
	w.Header().Set("Set-Cookie", argocdCookie.String())

	claims, err := h.verifyToken(tokenString)
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

	if argoCDSettings.OIDCConfig() == nil || argoCDSettings.OIDCConfig().LogoutURL == "" || issuer == session.SessionManagerClaimsIssuer {
		http.Redirect(w, r, logoutRedirectURL, http.StatusSeeOther)
	} else {
		oidcConfig = argoCDSettings.OIDCConfig()
		logoutURL := constructLogoutURL(oidcConfig.LogoutURL, tokenString, logoutRedirectURL)
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
	}
}
