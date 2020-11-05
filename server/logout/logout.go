package server

import (
	"net/http"
	"regexp"

	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/settings"
)

//NewHandler creates handler serving to do api/logout endpoint
func NewHandler(token string, appClientset versioned.Interface, settingsMrg *settings.SettingsManager, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg, token: token}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
	token        string
}

var (
	tokenPattern = regexp.MustCompile(`{{token}}`)
	token        string
)

func constructLogoutURL(logoutURL, token string) string {
	return tokenPattern.ReplaceAllString(logoutURL, token)
}

//ServeHTTP constructs the logout URL for OIDC provider by using the ID token
//and redirect URL and invalidates the OIDC session on logout from argoCD
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
	}

	OIDCConfig := argoCDSettings.OIDCConfig()
	logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, h.token)

	_, err = http.Get(logoutURL)

}
