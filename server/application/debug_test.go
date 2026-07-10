package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestDebugHandler_ServeHTTP_missing_params(t *testing.T) {
	t.Parallel()

	testKeys := []string{
		"pod",
		"appName",
		"projectName",
		"namespace",
		"image",
	}

	// test both empty and missing values
	testValues := []string{""}

	for _, testKey := range testKeys {
		testKeyCopy := testKey

		for _, testValue := range testValues {
			testValueCopy := testValue

			t.Run(testKeyCopy+" "+testValueCopy, func(t *testing.T) {
				t.Parallel()

				handler := debugHandler{}
				params := map[string]string{
					"pod":         "valid",
					"appName":     "valid",
					"projectName": "valid",
					"namespace":   "valid",
					"image":       "busybox:latest",
				}
				params[testKeyCopy] = testValueCopy
				var paramsArray []string
				for key, value := range params {
					paramsArray = append(paramsArray, key+"="+value)
				}
				paramsString := strings.Join(paramsArray, "&")
				request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug?"+paramsString, http.NoBody)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				response := recorder.Result()
				assert.Equal(t, http.StatusBadRequest, response.StatusCode)
			})
		}
	}
}

func TestDebugHandler_ServeHTTP_invalid_params(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		paramKey   string
		paramValue string
	}{
		{name: "invalid pod name", paramKey: "pod", paramValue: "invalid%20name"},
		{name: "invalid app name", paramKey: "appName", paramValue: "invalid%20name"},
		{name: "invalid project name", paramKey: "projectName", paramValue: "invalid%20name"},
		{name: "invalid namespace", paramKey: "namespace", paramValue: "invalid%20name"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := debugHandler{}
			params := map[string]string{
				"pod":         "valid",
				"appName":     "valid",
				"projectName": "valid",
				"namespace":   "valid",
				"image":       "busybox:latest",
			}
			params[tc.paramKey] = tc.paramValue
			var paramsArray []string
			for key, value := range params {
				paramsArray = append(paramsArray, key+"="+value)
			}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug?"+strings.Join(paramsArray, "&"), http.NoBody)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)
		})
	}
}

func TestDebugHandler_ServeHTTP_disallowed_namespace(t *testing.T) {
	handler := debugHandler{namespace: "argocd", enabledNamespaces: []string{"allowed"}}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug?pod=valid&appName=valid&projectName=valid&namespace=test&image=busybox:latest&appNamespace=disallowed", http.NoBody)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
	assert.Equal(t, security.NamespaceNotPermittedError("disallowed").Error()+"\n", recorder.Body.String())
}

func TestDebugHandler_WithFeatureFlagMiddleware_disabled(t *testing.T) {
	handler := &debugHandler{
		getSettings: func() (*settings.ArgoCDSettings, error) {
			return &settings.ArgoCDSettings{DebugEnabled: false}, nil
		},
	}
	middleware := handler.WithFeatureFlagMiddleware()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug", http.NoBody)
	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNotFound, recorder.Result().StatusCode)
}

func TestDebugHandler_WithFeatureFlagMiddleware_enabled(t *testing.T) {
	called := false
	handler := &debugHandler{
		getSettings: func() (*settings.ArgoCDSettings, error) {
			return &settings.ArgoCDSettings{
				DebugEnabled: true,
				DebugImages:  []string{"busybox:latest"},
			}, nil
		},
	}
	// Override ServeHTTP to track if it was called by embedding a custom handler
	// We test that when enabled, the request proceeds past the middleware (gets a non-404 response)
	middleware := handler.WithFeatureFlagMiddleware()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug?pod=valid&appName=valid&projectName=valid&namespace=valid&image=busybox:latest", http.NoBody)
	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, request)
	// Feature flag is enabled - should NOT return 404
	assert.NotEqual(t, http.StatusNotFound, recorder.Result().StatusCode)
	_ = called
}

func TestEnforceDebugAccess(t *testing.T) {
	t.Parallel()

	enf := rbac.NewEnforcer(fake.NewClientset(), "argocd", "argocd-rbac-cm", nil)
	policy := `
p, role:bare, exec, debug, */*, allow
p, role:scoped, exec, debug/docker.io/library/busybox:*, */*, allow
g, alice, role:bare
g, bob, role:scoped
`
	require.NoError(t, enf.SetUserPolicy(policy))

	const appRBACName = "myproj/myapp"

	testCases := []struct {
		name    string
		subject string
		image   string
		allowed bool
	}{
		{name: "bare grant allows any image", subject: "alice", image: "redis:7", allowed: true},
		{name: "scoped grant allows matching image", subject: "bob", image: "docker.io/library/busybox:1.36", allowed: true},
		{name: "scoped grant rejects other image", subject: "bob", image: "redis:7", allowed: false},
		{name: "no grant rejects", subject: "carol", image: "docker.io/library/busybox:1.36", allowed: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.allowed, enforceDebugAccess(enf, tc.subject, appRBACName, tc.image))
		})
	}
}
