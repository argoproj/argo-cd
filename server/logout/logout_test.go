package logout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-cd/common"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	baseLogoutURL                        = "http://localhost:4000/logout"
	baseLogoutURLwithToken               = "http://localhost:4000/logout?id_token_hint={{token}}"
	baseLogoutURLwithRedirectURL         = "http://localhost:4000/logout?post_logout_redirect_uri={{logoutRedirectURL}}"
	baseLogoutURLwithTokenAndRedirectURL = "http://localhost:4000/logout?id_token_hint={{token}}&post_logout_redirect_uri={{logoutRedirectURL}}"
	token                                = "sample-token"
	logoutRedirectURL                    = "http://localhost:4000/login"
	constructedLogoutURL                 = "http://localhost:4000/logout?id_token_hint=" + token + "&post_logout_redirect_uri=" + logoutRedirectURL
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
			token:                token,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: baseLogoutURL,
		},
		{

			name:                 "Case: ID token passed to logout URL",
			logoutURL:            baseLogoutURLwithToken,
			token:                token,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + token,
		},
		{

			name:                 "Case: Redirect URL passed to logout URL",
			logoutURL:            baseLogoutURLwithRedirectURL,
			token:                token,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?post_logout_redirect_uri=" + logoutRedirectURL,
		},
		{

			name:                 "Case: ID token and redirect URL passed to logout URL",
			logoutURL:            baseLogoutURLwithTokenAndRedirectURL,
			token:                token,
			logoutRedirectURL:    logoutRedirectURL,
			constructedLogoutURL: "http://localhost:4000/logout?id_token_hint=" + token + "&post_logout_redirect_uri=" + logoutRedirectURL,
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

	header := make(map[string][]string)
	header["Cookie"] = []string{"argocd.token=" + token}

	validRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)
	// Setting cookie in request
	validRequest.Header = header

	invalidRequest, err := http.NewRequest("GET", "http://localhost:4000/api/logout", nil)
	assert.NoError(t, err)

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
