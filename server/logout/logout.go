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

	// argoCDSettings, err := h.settingsMgr.GetSettings()
	// if err != nil {
	// }

	// OIDCConfig := argoCDSettings.OIDCConfig()
	// logoutURL := constructLogoutURL(OIDCConfig.LogoutURL, tokenString)

	// // _, err = http.Get(logoutURL)
	// fmt.Printf(logoutURL)
	_, _ = http.Get("http://golang.org/")
	http.Redirect(w, r, "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint=eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA0NTEwNTc2LCJleHAiOjE2MDQ1MTQxNzYsImp0aSI6IklELi1zVk14bGVMUjdQMzdQUURhdmtkTXJmcmxyZzdCb0Z5UjFZckVZT1BsSFEiLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDQ1MDUwOTgsImF0X2hhc2giOiJ3aUo0dFFkZTg5ME1vTTd5eVlJU2xRIn0.Nh6qUIb7IhaGoefYql0Dp40DIH3JCsGT-kw6tds4s9gxwdCG9FNKgpV7dBC0U9KOd2WX8vKreb_-OE2q7ze01xpnt1yYpVvgZpLYmkHSDJJBwM-WaCfCxaBsNUZx7HFZSw_RFscMJ5Gw1vd-JJm151XSzVKcPlHi65arC-W6ueI4BbzVWXOGhU1miocwV1PWgxyqn1inn5_GJoXdvUm2iIL3xPOz3NCFKSJ_L7PDSxqd7KHh0hynykaCeYE2F0-g2V67rsfUfgoM6oPXxDvKmSWDn-6gOH5AZiTysKxoEb0b_IludiN366AvF4Naty7n0GjR81ivtNkq2AclqGiZkQ", http.StatusSeeOther)

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
