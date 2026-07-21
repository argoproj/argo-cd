package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/configbus"
)

func testServerWithRootPath(rootPath string) *ArgoCDServer {
	server := &ArgoCDServer{
		ArgoCDServerOpts: ArgoCDServerOpts{
			RootPath: rootPath,
		},
	}
	server.configProvider = &configbus.StaticProvider{Fields: configbus.StaticFields{
		RootPath: configbus.Ptr(rootPath),
	}}
	return server
}

// TestWithRootPathEmptyRootPath tests that withRootPath returns the original handler when RootPath is empty
func TestWithRootPathEmptyRootPath(t *testing.T) {
	t.Parallel()
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := testServerWithRootPath("")
	handler, err := withRootPath(originalHandler, server)
	require.NoError(t, err)

	_, isServeMux := handler.(*http.ServeMux)
	assert.False(t, isServeMux, "When RootPath is empty, withRootPath should return the original handler, not a ServeMux")
}

// TestWithRootPathNonEmptyRootPath tests that withRootPath returns a ServeMux when RootPath is not empty
func TestWithRootPathNonEmptyRootPath(t *testing.T) {
	t.Parallel()
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := testServerWithRootPath("/argocd")
	handler, err := withRootPath(originalHandler, server)
	require.NoError(t, err)

	_, isServeMux := handler.(*http.ServeMux)
	assert.True(t, isServeMux, "When RootPath is not empty, withRootPath should return a ServeMux")
}

// TestNewRedirectServerEmptyRootPath tests that newRedirectServer correctly handles empty rootPath
func TestNewRedirectServerEmptyRootPath(t *testing.T) {
	t.Parallel()
	server := newRedirectServer(8080, "")

	assert.Equal(t, "localhost:8080", server.Addr, "When rootPath is empty, server address should be 'localhost:8080'")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")
	expectedURL := "https://example.com:8080/applications"
	assert.Equal(t, expectedURL, redirectURL, "Redirect URL should not include rootPath when rootPath is empty")
}

// TestNewRedirectServerNonEmptyRootPath tests that newRedirectServer correctly handles non-empty rootPath
func TestNewRedirectServerNonEmptyRootPath(t *testing.T) {
	t.Parallel()
	server := newRedirectServer(8080, "/argocd")

	assert.Equal(t, "localhost:8080/argocd", server.Addr, "When rootPath is '/argocd', server address should be 'localhost:8080/argocd'")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")
	expectedURL := "https://example.com:8080/argocd/applications"
	assert.Equal(t, expectedURL, redirectURL, "Redirect URL should include rootPath when rootPath is not empty")
}

// TestNewRedirectServerRootPathDuplication tests that newRedirectServer does not duplicate rootPath in the redirect URL
func TestNewRedirectServerRootPathDuplication(t *testing.T) {
	t.Parallel()
	server := newRedirectServer(8080, "/argocd")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/argocd/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")

	duplicatedURL := "https://example.com:8080/argocd/argocd/applications"
	assert.NotEqual(t, duplicatedURL, redirectURL, "Redirect URL should not have duplicated rootPath")

	correctURL := "https://example.com:8080/argocd/applications"
	assert.Equal(t, correctURL, redirectURL, "Redirect URL should be correct without duplicated rootPath")
}
