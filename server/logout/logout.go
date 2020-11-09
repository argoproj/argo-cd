package server

import (
	"fmt"
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
	token                    string
	tokenPrefix              = common.AuthCookieName + "="
)

func constructLogoutURL(logoutURL, token, logoutRedirectURL string) string {
	constructedLogoutURL := tokenPattern.ReplaceAllString(logoutURL, token)
	return logoutRedirectURLPattern.ReplaceAllString(constructedLogoutURL, logoutRedirectURL)
}

//ServeHTTP constructs the logout URL for OIDC provider by using the ID token
//and redirect URL and invalidates the OIDC session on logout from argoCD
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var tokenString string
	for _, cookie := range r.Header["Cookie"] {
		if strings.HasPrefix(cookie, tokenPrefix) {
			tokenString = fmt.Sprintf("%s", tokenNamePattern.ReplaceAllString(cookie, ""))
		}
	}
	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
	}

	OIDCConfig := argoCDSettings.OIDCConfig()
	logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString, OIDCConfig.LogoutRedirectURL)

	w.Write([]byte(logoutURL))
}
