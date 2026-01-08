package logout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	validJWTPattern                      = regexp.MustCompile(`[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+`)
	baseURL                              = "http://localhost:4000"
	rootPath                             = "argocd"
	baseHRef                             = "argocd"
	baseLogoutURL                        = "http://localhost:4000/logout"
	baseLogoutURLwithToken               = "http://localhost:4000/logout?id_token_hint={{token}}"
	baseLogoutURLwithRedirectURL         = "http://localhost:4000/logout?post_logout_redirect_uri={{logoutRedirectURL}}"
	baseLogoutURLwithTokenAndRedirectURL = "http://localhost:4000/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}"
	invalidToken                         = "sample-token"
	dexToken                             = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsImtpZCI6Ijc5YTFlNTgzOWM2ZTRjMTZlOTIwYzM1YTU0MmMwNmZhIn0.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA1NTcyMzU5LCJleHAiOjE2MDU1NzU5NTksImFtciI6WyJwd2QiXSwiaWRwIjoiMDBvaWdoZmZ2SlFMNjNaOGg1ZDUiLCJwcmVmZXJyZWRfdXNlcm5hbWUiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsImF1dGhfdGltZSI6MTYwNTU3MjM1NywiYXRfaGFzaCI6ImplUTBGaXZqT2c0YjZNSldEMjE5bGcifQ.Xt_5G-4dNZef1egOYmvruszudlAvUXVQzqrI4YwkWJeZ0zZDk4lyhPUVuxVGjB3pCCUCUMloTL6xC7IVFNj53Eb7WNH_hxsFqemJ80HZYbUpo2G9fMjkPmFTaeFVMC4p3qxIaBAT9_uJbTRSyRGYLV-95KDpU-GNDFXlbFq-2bVvhppiYmKszyHbREZkB87Pi7K3Bk0NxAlDOJ7O5lhwjpwuOJ1WGCJptUetePm5MnpVT2ZCyjvntlzwHlIhMSKNlFZuFS_JMca5Ww0fQSBUlarQU9MMyZKBw-QuD5sJw3xjwQpxOG-T9mJz7F8VA5znLi_LJNutHVgcpt3T_TW_0NbgqsHe8Lw"
	oidcToken                            = "eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA1NTcyMzU5LCJleHAiOjE2MDU1NzU5NTksImp0aSI6IklELl9ORDJxVG5iREFtc3hIZUt2U2ZHeVBqTXRicXFEQXdkdlRQTDZCTnpfR3ciLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDU1NzIzNTcsImF0X2hhc2giOiJqZVEwRml2ak9nNGI2TUpXRDIxOWxnIn0.GHkqwXgW-lrAhJdypW7SVjW0YdNLFQiRL8iwgT6DHJxP9Nb0OtkH2NKcBYAA5N6bTPLRQUHgYwWcgm5zSXmvqa7ciIgPF3tiQI8UmJA9VFRRDR-x9ExX15nskCbXfiQ67MriLslUrQUyzSCfUrSjXKwnDxbKGQncrtmRsh5asfCzJFb9excn311W9HKbT3KA0Ot7eOMnVS6V7SGfXxnKs6szcXIEMa_FhB4zDAVLr-dnxvSG_uuWcHrAkLTUVhHbdQQXF7hXIEfyr5lkMJN-drjdz-bn40GaYulEmUvO1bjcL9toCVQ3Ismypyr0b8phj4w3uRsLDZQxTxK7jAXlyQ"
	nonOidcToken                         = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2MDU1NzQyMTIsImlzcyI6ImFyZ29jZCIsIm5iZiI6MTYwNTU3NDIxMiwic3ViIjoiYWRtaW4ifQ.zDJ4piwWnwsHON-oPusHMXWINlnrRDTQykYogT7afeE"
	expectedNonOIDCLogoutURL             = "http://localhost:4000"
	expectedDexLogoutURL                 = "http://localhost:4000"
	expectedNonOIDCLogoutURLOnSecondHost = "http://argocd.my-corp.tld"
	expectedOIDCLogoutURL                = "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint=" + oidcToken + "&post_logout_redirect_uri=" + baseURL
	expectedOIDCLogoutURLWithRootPath    = "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint=" + oidcToken + "&post_logout_redirect_uri=" + baseURL + "/" + rootPath
)

func TestConstructLogoutURL(t *testing.T) {
	tests := []struct {
		name              string
		logoutURL         string
		token             string
		logoutRedirectURL string
		expectedLogoutURL string
	}{
		{
			name:              "Case: No additional parameters passed to logout URL",
			logoutURL:         baseLogoutURL,
			token:             oidcToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: baseLogoutURL,
		},
		{
			name:              "Case: ID token passed to logout URL",
			logoutURL:         baseLogoutURLwithToken,
			token:             oidcToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + oidcToken,
		},
		{
			name:              "Case: Redirect required",
			logoutURL:         baseLogoutURLwithRedirectURL,
			token:             oidcToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?post_logout_redirect_uri=" + baseURL,
		},
		{
			name:              "Case: ID token and redirect URL passed to logout URL",
			logoutURL:         baseLogoutURLwithTokenAndRedirectURL,
			token:             oidcToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + oidcToken + "&post_logout_redirect_uri=" + baseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constructedLogoutURL := constructLogoutURL(tt.logoutURL, tt.token, tt.logoutRedirectURL)
			assert.Equal(t, tt.expectedLogoutURL, constructedLogoutURL)
		})
	}
}

func TestHandlerConstructLogoutURL(t *testing.T) {
	kubeClientWithDexConfig := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"dex.config": "connectors: \n" +
					"- type: dev \n" +
					"name: Dev \n" +
					"config: \n" +
					"issuer: https://dev-5695098.okta.com \n" +
					"clientID: aabbccddeeff00112233 \n" +
					"clientSecret: aabbccddeeff00112233",
				"url": "http://localhost:4000",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	kubeClientWithOIDCConfig := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"oidc.config": "name: Okta \n" +
					"issuer: https://dev-5695098.okta.com \n" +
					"requestedScopes: [\"openid\", \"profile\", \"email\", \"groups\"] \n" +
					"requestedIDTokenClaims: {\"groups\": {\"essential\": true}} \n" +
					"logoutURL: https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}",
				"url": "http://localhost:4000",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	kubeClientWithOIDCConfigButNoURL := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"oidc.config": "name: Okta \n" +
					"issuer: https://dev-5695098.okta.com \n" +
					"requestedScopes: [\"openid\", \"profile\", \"email\", \"groups\"] \n" +
					"requestedIDTokenClaims: {\"groups\": {\"essential\": true}} \n" +
					"logoutURL: https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}",
				"url": "",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	kubeClientWithOIDCConfigButNoLogoutURL := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"oidc.config": "name: Okta \n" +
					"issuer: https://dev-5695098.okta.com \n" +
					"requestedScopes: [\"openid\", \"profile\", \"email\", \"groups\"] \n" +
					"requestedIDTokenClaims: {\"groups\": {\"essential\": true}} \n",
				"url": "http://localhost:4000",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	kubeClientWithoutOIDCAndMultipleURLs := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"url":            "http://localhost:4000",
				"additionalUrls": "- http://argocd.my-corp.tld",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	kubeClientWithoutOIDCConfig := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"url": "http://localhost:4000",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)

	settingsManagerWithDexConfig := settings.NewSettingsManager(t.Context(), kubeClientWithDexConfig, "default")
	settingsManagerWithOIDCConfig := settings.NewSettingsManager(t.Context(), kubeClientWithOIDCConfig, "default")
	settingsManagerWithoutOIDCConfig := settings.NewSettingsManager(t.Context(), kubeClientWithoutOIDCConfig, "default")
	settingsManagerWithOIDCConfigButNoLogoutURL := settings.NewSettingsManager(t.Context(), kubeClientWithOIDCConfigButNoLogoutURL, "default")
	settingsManagerWithoutOIDCAndMultipleURLs := settings.NewSettingsManager(t.Context(), kubeClientWithoutOIDCAndMultipleURLs, "default")
	settingsManagerWithOIDCConfigButNoURL := settings.NewSettingsManager(t.Context(), kubeClientWithOIDCConfigButNoURL, "default")

	redisClient, closer := test.NewInMemoryRedis()
	defer closer()
	sessionManager := session.NewSessionManager(settingsManagerWithOIDCConfig, test.NewFakeProjLister(), "", nil, session.NewUserStateStorage(redisClient))

	dexHandler := NewHandler(settingsManagerWithDexConfig, sessionManager, rootPath, baseHRef)
	dexHandler.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: "dev"}, "", nil
	}
	oidcHandler := NewHandler(settingsManagerWithOIDCConfig, sessionManager, rootPath, baseHRef)
	oidcHandler.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: "okta"}, "", nil
	}
	nonoidcHandler := NewHandler(settingsManagerWithoutOIDCConfig, sessionManager, "", baseHRef)
	nonoidcHandler.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: session.SessionManagerClaimsIssuer}, "", nil
	}
	oidcHandlerWithoutLogoutURL := NewHandler(settingsManagerWithOIDCConfigButNoLogoutURL, sessionManager, "", baseHRef)
	oidcHandlerWithoutLogoutURL.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: "okta"}, "", nil
	}
	nonoidcHandlerWithMultipleURLs := NewHandler(settingsManagerWithoutOIDCAndMultipleURLs, sessionManager, "", baseHRef)
	nonoidcHandlerWithMultipleURLs.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: "okta"}, "", nil
	}

	oidcHandlerWithoutBaseURL := NewHandler(settingsManagerWithOIDCConfigButNoURL, sessionManager, "argocd", baseHRef)
	oidcHandlerWithoutBaseURL.verifyToken = func(_ context.Context, tokenString string) (jwt.Claims, string, error) {
		if !validJWTPattern.MatchString(tokenString) {
			return nil, "", errors.New("invalid jwt")
		}
		return &jwt.RegisteredClaims{Issuer: "okta"}, "", nil
	}

	dexTokenHeader := make(map[string][]string)
	dexTokenHeader["Cookie"] = []string{"argocd.token=" + dexToken}
	oidcTokenHeader := make(map[string][]string)
	oidcTokenHeader["Cookie"] = []string{"argocd.token=" + oidcToken}
	nonOidcTokenHeader := make(map[string][]string)
	nonOidcTokenHeader["Cookie"] = []string{"argocd.token=" + nonOidcToken}
	invalidHeader := make(map[string][]string)
	invalidHeader["Cookie"] = []string{"argocd.token=" + invalidToken}
	emptyHeader := make(map[string][]string)
	emptyHeader["Cookie"] = []string{"argocd.token="}
	ctx := t.Context()

	dexRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)
	dexRequest.Header = dexTokenHeader
	oidcRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)
	oidcRequest.Header = oidcTokenHeader
	nonoidcRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)
	nonoidcRequest.Header = nonOidcTokenHeader
	nonoidcRequestOnSecondHost, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://argocd.my-corp.tld/api/logout", http.NoBody)
	require.NoError(t, err)
	nonoidcRequestOnSecondHost.Header = nonOidcTokenHeader
	require.NoError(t, err)
	requestWithInvalidToken, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)
	requestWithInvalidToken.Header = invalidHeader
	requestWithEmptyToken, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)
	requestWithEmptyToken.Header = emptyHeader

	invalidRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:4000/api/logout", http.NoBody)
	require.NoError(t, err)

	tests := []struct {
		name              string
		kubeClient        *fake.Clientset
		handler           http.Handler
		request           *http.Request
		responseRecorder  *httptest.ResponseRecorder
		expectedLogoutURL string
		wantErr           bool
	}{
		{
			name:              "Case: Dex logout request with valid token",
			handler:           dexHandler,
			request:           dexRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedDexLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case: OIDC logout request with valid token",
			handler:           oidcHandler,
			request:           oidcRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedOIDCLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case: OIDC logout request with valid token but missing URL",
			handler:           oidcHandlerWithoutBaseURL,
			request:           oidcRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedOIDCLogoutURLWithRootPath,
			wantErr:           false,
		},
		{
			name:              "Case: non-OIDC logout request with valid token",
			handler:           nonoidcHandler,
			request:           nonoidcRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case: Logout request with invalid token",
			handler:           nonoidcHandler,
			request:           requestWithInvalidToken,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case: Logout request with empty token",
			handler:           nonoidcHandler,
			request:           requestWithEmptyToken,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           true,
		},
		{
			name:              "Case: Logout request with missing token",
			handler:           oidcHandler,
			request:           invalidRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           true,
		},
		{
			name:              "Case:OIDC Logout request with missing logout URL configuration in config map",
			handler:           oidcHandlerWithoutLogoutURL,
			request:           oidcRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case:non-OIDC Logout request on the first supported URL",
			handler:           nonoidcHandlerWithMultipleURLs,
			request:           nonoidcRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case:non-OIDC Logout request on the second supported URL",
			handler:           nonoidcHandlerWithMultipleURLs,
			request:           nonoidcRequestOnSecondHost,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURLOnSecondHost,
			wantErr:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.handler.ServeHTTP(tt.responseRecorder, tt.request)
			if status := tt.responseRecorder.Code; status != http.StatusSeeOther {
				if !tt.wantErr {
					t.Error(tt.responseRecorder.Body.String())
					t.Error("handler returned wrong status code: " + strconv.Itoa(tt.responseRecorder.Code))
				}
			} else {
				if tt.wantErr {
					t.Errorf("expected error but did not get one")
				} else {
					assert.Equal(t, tt.expectedLogoutURL, tt.responseRecorder.Result().Header["Location"][0])
				}
			}
		})
	}
}
