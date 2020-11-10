package logout

import (
	"net/http"
	"regexp"

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
	// for _, cookie := range r.Header["Cookie"] {
	// 	if strings.HasPrefix(cookie, tokenPrefix) {
	// 		tokenString = tokenNamePattern.ReplaceAllString(cookie, "")
	// 		break
	// 	}
	// }
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

		// logoutURL = "https://dev-5695099.okta.com/oauth2/v1/logout?id_token_hint=eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA0NjExMDg5LCJleHAiOjE2MDQ2MTQ2ODksImp0aSI6IklELjZEVlM2enluandLeFJuNkhNUkJIaXBJMDdmNUxQRzFHc054OTVScENlYmciLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDQ2MTEwODgsImF0X2hhc2giOiJKWjcwWUhsM3k5eWhpaXZaaU9OQTVRIn0.TauJ_MyIT3EtbkXfRvEdSh4H7YS6ezwO4mJyv5A1_ml9HkKsxWUht09U9T-VFJyqweOim_2fyRMsc6VCtAla9kCsNUHvHU1uEDrMageWePfIxrM0yQ2Fys2cbSl2dZGeTlX2I-xt9_EhZszLdKccdyhaBL1JYMTc0ajTWNmFN9azn-WJKkAwDe-3EUMYm9hfYSLkrMqqsExPNCQlc0LMgcWPzj3gQwCYj1MvMO3F3U99i5FglIHjw99sC7StMEHOfKzuCwIuceNfqLXHZ0GpbDgDfYpfn4JksCfXsWJd2niWGeOULAgl1-vk1WUt3K5qKFCz0HGGPfLMjqkdcGTQ_A"
		_, _ = w.Write([]byte(logoutURL))
	}

}
