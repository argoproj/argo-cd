package logout

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/settings"
)

//NewHandler creates handler serving to do api/logout endpoint
func NewHandler(appClientset versioned.Interface, settingsMrg *settings.SettingsManager, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
}

var (
	tokenPattern             = regexp.MustCompile(`{{token}}`)
	logoutRedirectURLPattern = regexp.MustCompile(`{{logoutRedirectURL}}`)
	tokenNamePattern         = regexp.MustCompile(tokenPrefix)
	tokenPrefix              = common.AuthCookieName + "="
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
	for _, cookie := range r.Header["Cookie"] {
		if strings.HasPrefix(cookie, tokenPrefix) {
			tokenString = tokenNamePattern.ReplaceAllString(cookie, "")
			break
		}
	}
	if tokenString == "" {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to retrieve auth token", http.StatusInternalServerError)
	}

	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
	}

	if argoCDSettings.OIDCConfig() == nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to retrieve oidc config settings", http.StatusInternalServerError)
	} else {
		OIDCConfig = argoCDSettings.OIDCConfig()
		logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString, OIDCConfig.LogoutRedirectURL)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(logoutURL))
	}

}
