package logout

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-cd/common"

	"github.com/argoproj/argo-cd/util/session"

	"github.com/argoproj/argo-cd/util/settings"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
)

var (
	baseURL                              = "http://localhost:4000"
	baseLogoutURL                        = "http://localhost:4000/logout"
	baseLogoutURLwithToken               = "http://localhost:4000/logout?id_token_hint={{token}}"
	baseLogoutURLwithRedirectURL         = "http://localhost:4000/logout?post_logout_redirect_uri={{logoutRedirectURL}}"
	baseLogoutURLwithTokenAndRedirectURL = "http://localhost:4000/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}"
	invalidToken                         = "sample-token"
	OIDCToken                            = "eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA1NTcyMzU5LCJleHAiOjE2MDU1NzU5NTksImp0aSI6IklELl9ORDJxVG5iREFtc3hIZUt2U2ZHeVBqTXRicXFEQXdkdlRQTDZCTnpfR3ciLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDU1NzIzNTcsImF0X2hhc2giOiJqZVEwRml2ak9nNGI2TUpXRDIxOWxnIn0.GHkqwXgW-lrAhJdypW7SVjW0YdNLFQiRL8iwgT6DHJxP9Nb0OtkH2NKcBYAA5N6bTPLRQUHgYwWcgm5zSXmvqa7ciIgPF3tiQI8UmJA9VFRRDR-x9ExX15nskCbXfiQ67MriLslUrQUyzSCfUrSjXKwnDxbKGQncrtmRsh5asfCzJFb9excn311W9HKbT3KA0Ot7eOMnVS6V7SGfXxnKs6szcXIEMa_FhB4zDAVLr-dnxvSG_uuWcHrAkLTUVhHbdQQXF7hXIEfyr5lkMJN-drjdz-bn40GaYulEmUvO1bjcL9toCVQ3Ismypyr0b8phj4w3uRsLDZQxTxK7jAXlyQ"
	nonOIDCToken                         = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2MDU1NzQyMTIsImlzcyI6ImFyZ29jZCIsIm5iZiI6MTYwNTU3NDIxMiwic3ViIjoiYWRtaW4ifQ.zDJ4piwWnwsHON-oPusHMXWINlnrRDTQykYogT7afeE"
	expectedNonOIDCLogoutURL             = "http://localhost:4000/login"
	expectedOIDCLogoutURL                = "https://dev-5695098.okta.com/oauth2/v1/logout?id_token_hint=" + OIDCToken + "&post_logout_redirect_uri=" + baseURL
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
			token:             OIDCToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: baseLogoutURL,
		},
		{

			name:              "Case: ID token passed to logout URL",
			logoutURL:         baseLogoutURLwithToken,
			token:             OIDCToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + OIDCToken,
		},
		{

			name:              "Case: Redirect required",
			logoutURL:         baseLogoutURLwithRedirectURL,
			token:             OIDCToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?post_logout_redirect_uri=" + baseURL,
		},
		{

			name:              "Case: ID token and redirect URL passed to logout URL",
			logoutURL:         baseLogoutURLwithTokenAndRedirectURL,
			token:             OIDCToken,
			logoutRedirectURL: baseURL,
			expectedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + OIDCToken + "&post_logout_redirect_uri=" + baseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constructedLogoutURL := constructLogoutURL(tt.logoutURL, tt.token, tt.logoutRedirectURL)
			assert.Equal(t, constructedLogoutURL, tt.expectedLogoutURL)
		})
	}
}

func TestHandlerConstructLogoutURL(t *testing.T) {

	kubeClientWithOIDCConfig := fake.NewSimpleClientset(
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

	kubeClientWithoutOIDCConfig := fake.NewSimpleClientset(
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

	settingsManagerWithOIDCConfig := settings.NewSettingsManager(context.Background(), kubeClientWithOIDCConfig, "default")
	settingsManagerWithoutOIDCConfig := settings.NewSettingsManager(context.Background(), kubeClientWithoutOIDCConfig, "default")

	sessionManagerWithOIDCCOnfig := session.NewSessionManager(settingsManagerWithOIDCConfig, "", session.NewInMemoryUserStateStorage())
	sessionManagerWithoutOIDCConfig := session.NewSessionManager(settingsManagerWithoutOIDCConfig, "", session.NewInMemoryUserStateStorage())

	OIDCHandler := NewHandler(appclientset.NewSimpleClientset(), settingsManagerWithOIDCConfig, sessionManagerWithOIDCCOnfig, "", "default")
	nonOIDCHandler := NewHandler(appclientset.NewSimpleClientset(), settingsManagerWithoutOIDCConfig, sessionManagerWithoutOIDCConfig, "", "default")

	OIDCTokenHeader := make(map[string][]string)
	OIDCTokenHeader["Cookie"] = []string{"argocd.token=" + OIDCToken}
	nonOIDCTokenHeader := make(map[string][]string)
	nonOIDCTokenHeader["Cookie"] = []string{"argocd.token=" + nonOIDCToken}
	invalidHeader := make(map[string][]string)
	invalidHeader["Cookie"] = []string{"argocd.token=" + invalidToken}

	OIDCRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	OIDCRequest.Header = OIDCTokenHeader
	nonOIDCRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	nonOIDCRequest.Header = nonOIDCTokenHeader
	assert.NoError(t, err)
	requestWithInvalidToken, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	requestWithInvalidToken.Header = invalidHeader
	invalidRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)

	tests := []struct {
		name              string
		kubeClient        *fake.Clientset
		handler           http.Handler
		request           *http.Request
		responseRecorder  *httptest.ResponseRecorder
		expectedLogoutURL string
		wantErr           bool
	}{
		// {
		// 	name:              "Case: OIDC logout request with valid token",
		// 	handler:           OIDCHandler,
		// 	request:           OIDCRequest,
		// 	responseRecorder:  httptest.NewRecorder(),
		// 	expectedLogoutURL: expectedOIDCLogoutURL,
		// 	wantErr:           false,
		// },
		// {
		// 	name:              "Case: non-OIDC logout request with valid token",
		// 	handler:           nonOIDCHandler,
		// 	request:           nonOIDCRequest,
		// 	responseRecorder:  httptest.NewRecorder(),
		// 	expectedLogoutURL: expectedNonOIDCLogoutURL,
		// 	wantErr:           false,
		// },
		{
			name:              "Case: Logout request with invalid token",
			handler:           nonOIDCHandler,
			request:           requestWithInvalidToken,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedNonOIDCLogoutURL,
			wantErr:           true,
		},
		{
			name:              "Case: Logout request with missing token",
			handler:           OIDCHandler,
			request:           invalidRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: expectedOIDCLogoutURL,
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.handler.ServeHTTP(tt.responseRecorder, tt.request)
			if status := tt.responseRecorder.Code; status != http.StatusSeeOther {
				if !tt.wantErr {
					t.Errorf(tt.responseRecorder.Body.String())
					t.Errorf("handler returned wrong status code: " + fmt.Sprintf("%d", tt.responseRecorder.Code))
				}
			} else {
				if tt.wantErr {
					t.Errorf("expected error but did not get one")
				} else {
					assert.Equal(t, tt.expectedLogoutURL, tt.responseRecorder.HeaderMap.Get("Location"))
				}

			}
		})
	}

}
