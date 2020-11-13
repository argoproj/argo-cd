package logout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-cd/common"

	"github.com/argoproj/argo-cd/util/settings"

	"github.com/stretchr/testify/assert"

	"k8s.io/client-go/kubernetes/fake"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	baseLogoutURL                        = "http://localhost:4000/logout"
	baseLogoutURLwithToken               = "http://localhost:4000/logout?id_token_hint={{token}}"
	baseLogoutURLwithRedirectURL         = "http://localhost:4000/logout?post_logout_redirect_uri={{logoutRedirectURL}}"
	baseLogoutURLwithTokenAndRedirectURL = "http://localhost:4000/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}"
	invalidToken                         = "sample-token"
	validToken                           = "eyJraWQiOiJYQi1MM3ZFdHhYWXJLcmRSQnVEV0NwdnZsSnk3SEJVb2d5N253M1U1Z1ZZIiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIwMHVqNnM1NDVyNU5peVNLcjVkNSIsIm5hbWUiOiJqZCByIiwiZW1haWwiOiJqYWlkZWVwMTdydWx6QGdtYWlsLmNvbSIsInZlciI6MSwiaXNzIjoiaHR0cHM6Ly9kZXYtNTY5NTA5OC5va3RhLmNvbSIsImF1ZCI6IjBvYWowM2FmSEtqN3laWXJwNWQ1IiwiaWF0IjoxNjA0NjExMDg5LCJleHAiOjE2MDQ2MTQ2ODksImp0aSI6IklELjZEVlM2enluandLeFJuNkhNUkJIaXBJMDdmNUxQRzFHc054OTVScENlYmciLCJhbXIiOlsicHdkIl0sImlkcCI6IjAwb2lnaGZmdkpRTDYzWjhoNWQ1IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiamFpZGVlcDE3cnVsekBnbWFpbC5jb20iLCJhdXRoX3RpbWUiOjE2MDQ2MTEwODgsImF0X2hhc2giOiJKWjcwWUhsM3k5eWhpaXZaaU9OQTVRIn0.TauJ_MyIT3EtbkXfRvEdSh4H7YS6ezwO4mJyv5A1_ml9HkKsxWUht09U9T-VFJyqweOim_2fyRMsc6VCtAla9kCsNUHvHU1uEDrMageWePfIxrM0yQ2Fys2cbSl2dZGeTlX2I-xt9_EhZszLdKccdyhaBL1JYMTc0ajTWNmFN9azn-WJKkAwDe-3EUMYm9hfYSLkrMqqsExPNCQlc0LMgcWPzj3gQwCYj1MvMO3F3U99i5FglIHjw99sC7StMEHOfKzuCwIuceNfqLXHZ0GpbDgDfYpfn4JksCfXsWJd2niWGeOULAgl1-vk1WUt3K5qKFCz0HGGPfLMjqkdcGTQ_A"
	logoutRedirectURL                    = "http://localhost:4000/login"
	constructedLogoutURL                 = "http://localhost:4000/logout?id_token_hint=" + validToken + "&post_logout_redirect_uri=" + logoutRedirectURL
)

func TestConstructLogoutURL(t *testing.T) {

	tests := []struct {
		name                 string
		logoutURL            string
		token                string
		logoutRedirectURL    string
		constructedLogoutURL string
	}{
		{
			name:                 "Case: No additional parameters passed to logout URL",
			logoutURL:            baseLogoutURL,
			token:                validToken,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: baseLogoutURL,
		},
		{

			name:                 "Case: ID token passed to logout URL",
			logoutURL:            baseLogoutURLwithToken,
			token:                validToken,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + validToken,
		},
		{

			name:                 "Case: Redirect URL passed to logout URL",
			logoutURL:            baseLogoutURLwithRedirectURL,
			token:                validToken,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?post_logout_redirect_uri=" + logoutRedirectURL,
		},
		{

			name:                 "Case: ID token and redirect URL passed to logout URL",
			logoutURL:            baseLogoutURLwithTokenAndRedirectURL,
			token:                validToken,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + validToken + "&post_logout_redirect_uri=" + logoutRedirectURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constructedLogoutURL := constructLogoutURL(tt.logoutURL, tt.token, tt.logoutRedirectURL)
			assert.Equal(t, constructedLogoutURL, tt.constructedLogoutURL)
		})
	}
}

func TestHandlerConstructLogoutURL(t *testing.T) {

	kubeClientWithValidOIDCConfig := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"oidc.config": "\n  logoutURL: " + baseLogoutURLwithTokenAndRedirectURL + "\n  logoutRedirectURL: " + logoutRedirectURL + "\n",
			},
		},
		&v1.Secret{
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

	kubeClientWithInvalidOIDCConfig := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{},
		},
		&v1.Secret{
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

	settingsManager := settings.NewSettingsManager(context.Background(), kubeClientWithValidOIDCConfig, "default")
	settingsManagerWithoutValidConfigMap := settings.NewSettingsManager(context.Background(), kubeClientWithInvalidOIDCConfig, "default")

	validHandler := NewHandler(appclientset.NewSimpleClientset(), settingsManager, "default")
	handlerWithInvalidSettingsManager := NewHandler(appclientset.NewSimpleClientset(), settingsManagerWithoutValidConfigMap, "default")

	validHeader := make(map[string][]string)
	validHeader["Cookie"] = []string{"argocd.token=" + validToken}

	invalidHeader := make(map[string][]string)
	invalidHeader["Cookie"] = []string{"argocd.token=" + invalidToken}

	validRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	// Setting cookie in request
	validRequest.Header = validHeader

	invalidRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)

	RequestWithInvalidToken, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	RequestWithInvalidToken.Header = invalidHeader

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
			name:              "Case: valid oidc config available and cookie present in http request to handler",
			handler:           validHandler,
			request:           validRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: constructedLogoutURL,
			wantErr:           false,
		},
		{
			name:              "Case: valid oidc config unavailable and cookie present in http request to handler",
			handler:           handlerWithInvalidSettingsManager,
			request:           validRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: constructedLogoutURL,
			wantErr:           true,
		},
		{
			name:              "Case: valid oid config available but cookie not present in http request to handler",
			handler:           validHandler,
			request:           invalidRequest,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: constructedLogoutURL,
			wantErr:           true,
		},
		{
			name:              "Case: valid oid config available but token present in cookie doesnt not satisfy required format",
			handler:           validHandler,
			request:           RequestWithInvalidToken,
			responseRecorder:  httptest.NewRecorder(),
			expectedLogoutURL: constructedLogoutURL,
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.handler.ServeHTTP(tt.responseRecorder, tt.request)
			if status := tt.responseRecorder.Code; status != http.StatusOK {
				if !tt.wantErr {
					t.Errorf("handler returned wrong status code")
				}
			} else {
				if tt.wantErr {
					t.Errorf("expected error but did not get one")
				} else {
					constructedLogoutURL := tt.responseRecorder.Body.String()
					assert.Equal(t, tt.expectedLogoutURL, constructedLogoutURL)
				}

			}
		})
	}

}
