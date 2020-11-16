package logout

import (
	"net/http"
	"reflect"
	"regexp"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

//NewHandler creates handler serving to do api/logout endpoint
func NewHandler(appClientset versioned.Interface, settingsMrg *settings.SettingsManager, sessionMgr *session.SessionManager, rootPath, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg, sessionMgr: sessionMgr, rootPath: rootPath}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
	sessionMgr   *session.SessionManager
	rootPath     string
}

var (
	tokenPattern             = regexp.MustCompile(`{{token}}`)
	logoutRedirectURLPattern = regexp.MustCompile(`{{logoutRedirectURL}}`)
)

func constructLogoutURL(logoutURL, token, logoutRedirectURL string) string {
	constructedLogoutURL := tokenPattern.ReplaceAllString(logoutURL, token)
	return logoutRedirectURLPattern.ReplaceAllString(constructedLogoutURL, logoutRedirectURL)
}

// ServeHTTP constructs the logout URL for OIDC provider by using the ID token and logout redirect url if present
// and returns it to the ui
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var tokenString string
	var OIDCConfig *settings.OIDCConfig

	argocdCookie, err := r.Cookie(common.AuthCookieName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		http.Error(w, "Failed to retrieve ArgoCD auth token", http.StatusBadRequest)
		return
	}

	tokenString = argocdCookie.Value

	claims, err := h.sessionMgr.VerifyToken(tokenString)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		http.Error(w, "Failed to verify token", http.StatusBadRequest)
		return
	}

	var issuer string
	if reflect.ValueOf(claims).Kind() == reflect.Map {
		issuer = reflect.ValueOf(claims).MapIndex(reflect.ValueOf("iss")).Interface().(string)
	} else if reflect.ValueOf(claims).Kind() == reflect.Ptr {
		issuer = reflect.Indirect(reflect.ValueOf(claims)).MapIndex(reflect.ValueOf("iss")).Interface().(string)
	}

	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to retrieve argoCD settings", http.StatusInternalServerError)
		return
	}

	if argoCDSettings.OIDCConfig() == nil || issuer == session.SessionManagerClaimsIssuer {
		redirectURL := argoCDSettings.URL + "/login"
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	} else {
		OIDCConfig = argoCDSettings.OIDCConfig()
		LogoutRedirectURL := argoCDSettings.URL + h.rootPath
		logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString, LogoutRedirectURL)
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
	}
}
