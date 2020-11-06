package server

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/argoproj/pkg/jwt/zjwt"
	"google.golang.org/grpc/metadata"
)

//NewHandler creates handler serving to do api/logout endpoint
func NewHandler(ctx context.Context, appClientset versioned.Interface, settingsMrg *settings.SettingsManager, namespace string) http.Handler {
	return &Handler{appClientset: appClientset, namespace: namespace, settingsMgr: settingsMrg, ctx: ctx}
}

type Handler struct {
	namespace    string
	appClientset versioned.Interface
	settingsMgr  *settings.SettingsManager
	ctx          context.Context
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
	md, _ := metadata.FromIncomingContext(h.ctx)
	tokenString := getToken(md)
	fmt.Printf("=================================== TOKEN STRING IN LOGOUT %s =============================\n", tokenString)
	var token = "eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA0Njc5MDY2LCJleHAiOjE2MDQ2ODI2NjYsImp0aSI6IklELmFVeVk1eUFxaW43OVcxZ3JYSEx5NlpvZnJObTFkNER5LUtPV0tOTmZkOE0iLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDQ2NzkwNjUsImF0X2hhc2giOiJNRXdXd0RuVkJPa3NuQTkyS0JLZlJ3In0.WtuKR4EW6l8mZWkbEAx37SGHumUgnkgaerxzXjsIhnksDa0cR33GAFHckq_z2LTsc0cf-ldbg3kYInASCkLJ62HlX7fE4G4_aJ85E95ZDx2fI3R55CxdLF1ixz7l8ryzKu3EBQjkKxKq1cqg-wmgtMSOG7LMpKcUQALeb_K76gU00DLgIJwT6Vl7H5aq3xE9TP42FhAj5-JsLxihlKaHEq6PCYFvQgh809rdMCTpepCiDeqPSi1cLpcRHh3IWpdhhSZvUWGLmiR_Z5Ro8XUV0mjzRVUAJVwVa3qKzWR-KeXjWTkbWamG0HALGPSEOAoe9kRr0ULsm_wyqPLDB8OS4g"
	// var Bearer = "Bearer " + token
	// argoCDSettings, err := h.settingsMgr.GetSettings()
	// if err != nil {
	// }

	// OIDCConfig := argoCDSettings.OIDCConfig()
	// logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString)

	// // _, err = http.Get(logoutURL)
	// fmt.Printf(logoutURL)
	// r.Header.Add("Authorization", token)
	http.Redirect(w, r, "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint="+token+"&post_logout_redirect_uri=http://localhost:4000/login", http.StatusSeeOther)

}

// getToken extracts the token from gRPC metadata or cookie headers
func getToken(md metadata.MD) string {
	// check the "token" metadata
	{
		tokens, ok := md[apiclient.MetaDataTokenKey]
		if ok && len(tokens) > 0 {
			return tokens[0]
		}
	}

	var tokens []string

	// looks for the HTTP header `Authorization: Bearer ...`
	for _, t := range md["authorization"] {
		if strings.HasPrefix(t, "Bearer ") {
			tokens = append(tokens, strings.TrimPrefix(t, "Bearer "))
		}
	}

	// check the HTTP cookie
	for _, t := range md["grpcgateway-cookie"] {
		header := http.Header{}
		header.Add("Cookie", t)
		request := http.Request{Header: header}
		token, err := request.Cookie(common.AuthCookieName)
		if err == nil {
			tokens = append(tokens, token.Value)
		}
	}

	for _, t := range tokens {
		value, err := zjwt.JWT(t)
		if err == nil {
			return value
		}
	}
	return ""
}
