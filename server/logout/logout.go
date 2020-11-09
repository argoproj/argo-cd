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
	tokenPattern     = regexp.MustCompile(`{{token}}`)
	tokenNamePattern = regexp.MustCompile(tokenPrefix)
	token            string
	tokenPrefix      = common.AuthCookieName + "="
)

func constructLogoutURL(logoutURL, token string) string {
	return tokenPattern.ReplaceAllString(logoutURL, token)
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
	// tokenString := tokenNamePattern.ReplaceAllString(r.Header["Cookie"], "")
	// fmt.Printf("EXTRACTED TOKEN: %s", tokenString)
	argoCDSettings, err := h.settingsMgr.GetSettings()
	if err != nil {
	}

	OIDCConfig := argoCDSettings.OIDCConfig()
	logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString)

	w.Write([]byte(logoutURL))

	// // _, err = http.Get(logoutURL)
	// fmt.Printf(logoutURL)
	// r.Header.Add("Authorization", token)
	// http.Redirect(w, r, "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint="+token+"&post_logout_redirect_uri=http://localhost:4000/login", http.StatusSeeOther)

}
