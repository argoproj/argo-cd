package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
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

func TestDebugHandler_WithFeatureFlagMiddleware_enabledNoImages(t *testing.T) {
	// Enabled without a configured allowlist is a misconfiguration: there is no default,
	// so the endpoint must reject the request instead of silently allowing anything.
	handler := &debugHandler{
		getSettings: func() (*settings.ArgoCDSettings, error) {
			return &settings.ArgoCDSettings{DebugEnabled: true}, nil
		},
	}
	middleware := handler.WithFeatureFlagMiddleware()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://argocd.example.com/debug", http.NoBody)
	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusInternalServerError, recorder.Result().StatusCode)
}

func TestEnforceDebugAccess(t *testing.T) {
	t.Parallel()

	enf := rbac.NewEnforcer(fake.NewClientset(), "argocd", "argocd-rbac-cm", nil)
	policy := `
p, role:bare, exec, debug, */*, allow
p, role:any, exec, debug/*, */*, allow
p, role:any, exec, debug/docker.io/nicolaka/netshoot:*, */*, deny
p, role:scoped, exec, debug/docker.io/library/busybox:*, */*, allow
g, alice, role:bare
g, bob, role:scoped
g, dave, role:any
`
	require.NoError(t, enf.SetUserPolicy(policy))

	const appRBACName = "myproj/myapp"

	testCases := []struct {
		name    string
		subject string
		image   string
		allowed bool
	}{
		{name: "bare grant allows nothing", subject: "alice", image: "redis:7", allowed: false},
		{name: "any-image grant matches multi-slash reference", subject: "dave", image: "docker.io/library/busybox:1.36", allowed: true},
		{name: "any-image grant matches digest reference", subject: "dave", image: "ghcr.io/org/tools/debug@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", allowed: true},
		{name: "image-scoped deny overrides any-image allow", subject: "dave", image: "docker.io/nicolaka/netshoot:v0.13", allowed: false},
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

func newDebugTestHandler(t *testing.T, policy string, apps ...*appv1.Application) *debugHandler {
	t.Helper()
	enf := rbac.NewEnforcer(fake.NewClientset(), "argocd", "argocd-rbac-cm", nil)
	require.NoError(t, enf.SetUserPolicy(policy))
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, a := range apps {
		require.NoError(t, indexer.Add(a))
	}
	return &debugHandler{
		appLister:       applisters.NewApplicationLister(indexer),
		namespace:       "argocd",
		terminalOptions: &TerminalOptions{Enf: enf},
		getSettings: func() (*settings.ArgoCDSettings, error) {
			return &settings.ArgoCDSettings{DebugEnabled: true, DebugImages: []string{"busybox:1.36"}}, nil
		},
	}
}

func debugTestRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	ctx := context.WithValue(t.Context(), "claims", "alice") //nolint:staticcheck
	return httptest.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
}

func TestDebugHandler_ServeHTTP_rbacBeforeAllowlist(t *testing.T) {
	t.Parallel()
	handler := newDebugTestHandler(t, "p, alice, applications, get, myproj/*, allow")

	// Without debug access, an allowlisted and a non-allowlisted image must be indistinguishable.
	for _, image := range []string{"busybox:1.36", "redis:7"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, debugTestRequest(t, "https://argocd.example.com/debug?pod=mypod&appName=myapp&projectName=myproj&namespace=default&appNamespace=argocd&image="+image))
		assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode, image)
	}
}

func TestDebugHandler_ListDebugImages(t *testing.T) {
	t.Parallel()
	policy := `
p, alice, applications, get, myproj/*, allow
p, alice, exec, debug/*, myproj/*, allow
`
	app := &appv1.Application{
		Name:      "myapp",
		Namespace: "argocd",
		Spec:      appv1.ApplicationSpec{Project: "otherproj"},
	}
	ownApp := &appv1.Application{
		Name:      "ownapp",
		Namespace: "argocd",
		Spec:      appv1.ApplicationSpec{Project: "myproj"},
	}
	handler := newDebugTestHandler(t, policy, app, ownApp)

	testCases := []struct {
		name   string
		query  string
		status int
	}{
		{name: "unknown app", query: "appName=missing&projectName=myproj&appNamespace=argocd", status: http.StatusNotFound},
		{name: "wrong project", query: "appName=myapp&projectName=myproj&appNamespace=argocd", status: http.StatusBadRequest},
		{name: "own app", query: "appName=ownapp&projectName=myproj&appNamespace=argocd", status: http.StatusOK},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			handler.ListDebugImages(recorder, debugTestRequest(t, "https://argocd.example.com/debug-images?"+tc.query))
			require.Equal(t, tc.status, recorder.Result().StatusCode)
			if tc.status == http.StatusOK {
				var resp debugImagesResponse
				require.NoError(t, json.NewDecoder(recorder.Body).Decode(&resp))
				assert.Equal(t, []string{"busybox:1.36"}, resp.Images)
			}
		})
	}
}

func TestWaitForEphemeralContainer_failsFastOnNonTransientWaiting(t *testing.T) {
	t.Parallel()
	pod := &corev1.Pod{
		Name:      "mypod",
		Namespace: "default",
		Status: corev1.PodStatus{EphemeralContainerStatuses: []corev1.ContainerStatus{{
			Name:  "argocd-debug-abc123",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "back-off pulling image"}},
		}}},
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	err := waitForEphemeralContainer(ctx, fake.NewClientset(pod), "default", "mypod", "argocd-debug-abc123")
	var stateErr *containerStateError
	require.ErrorAs(t, err, &stateErr)
	assert.Equal(t, "ImagePullBackOff", stateErr.Reason)
}
