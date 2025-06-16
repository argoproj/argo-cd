package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithRootPathEmptyRootPath tests that withRootPath returns the original handler when RootPath is empty
func TestWithRootPathEmptyRootPath(t *testing.T) {
	// Create a simple handler
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a server with empty RootPath
	server := &ArgoCDServer{
		ArgoCDServerOpts: ArgoCDServerOpts{
			RootPath: "",
		},
	}

	// Call withRootPath
	handler := withRootPath(originalHandler, server)

	// Verify that the returned handler is the original handler
	// Since we can't directly compare function references, we'll use a type assertion
	_, isServeMux := handler.(*http.ServeMux)
	assert.False(t, isServeMux, "When RootPath is empty, withRootPath should return the original handler, not a ServeMux")
}

// TestWithRootPathNonEmptyRootPath tests that withRootPath returns a ServeMux when RootPath is not empty
func TestWithRootPathNonEmptyRootPath(t *testing.T) {
	// Create a simple handler
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a server with non-empty RootPath
	server := &ArgoCDServer{
		ArgoCDServerOpts: ArgoCDServerOpts{
			RootPath: "/argocd",
		},
	}

	// Call withRootPath
	handler := withRootPath(originalHandler, server)

	// Verify that the returned handler is a ServeMux
	_, isServeMux := handler.(*http.ServeMux)
	assert.True(t, isServeMux, "When RootPath is not empty, withRootPath should return a ServeMux")
}

// TestNewRedirectServerEmptyRootPath tests that newRedirectServer correctly handles empty rootPath
func TestNewRedirectServerEmptyRootPath(t *testing.T) {
	// Call newRedirectServer with empty rootPath
	server := newRedirectServer(8080, "")

	// Verify the server address
	assert.Equal(t, "localhost:8080", server.Addr, "When rootPath is empty, server address should be 'localhost:8080'")

	// Test the redirect handler
	req := httptest.NewRequest(http.MethodGet, "/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	// Verify the redirect URL
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")
	expectedURL := "https://example.com:8080/applications"
	assert.Equal(t, expectedURL, redirectURL, "Redirect URL should not include rootPath when rootPath is empty")
}

// TestNewRedirectServerNonEmptyRootPath tests that newRedirectServer correctly handles non-empty rootPath
func TestNewRedirectServerNonEmptyRootPath(t *testing.T) {
	// Call newRedirectServer with non-empty rootPath
	server := newRedirectServer(8080, "/argocd")

	// Verify the server address
	assert.Equal(t, "localhost:8080/argocd", server.Addr, "When rootPath is '/argocd', server address should be 'localhost:8080/argocd'")

	// Test the redirect handler
	req := httptest.NewRequest(http.MethodGet, "/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	// Verify the redirect URL
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")
	expectedURL := "https://example.com:8080/argocd/applications"
	assert.Equal(t, expectedURL, redirectURL, "Redirect URL should include rootPath when rootPath is not empty")
}

// TestNewRedirectServerRootPathDuplication tests that newRedirectServer does not duplicate rootPath in the redirect URL
func TestNewRedirectServerRootPathDuplication(t *testing.T) {
	// Call newRedirectServer with non-empty rootPath
	server := newRedirectServer(8080, "/argocd")

	// Test the redirect handler with a request path that already includes rootPath
	req := httptest.NewRequest(http.MethodGet, "/argocd/applications", http.NoBody)
	req.Host = "example.com:8080"
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	// Verify the redirect URL
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code, "Should return a 307 Temporary Redirect status code")
	redirectURL := w.Header().Get("Location")

	// The URL should not have duplicated rootPath
	duplicatedURL := "https://example.com:8080/argocd/argocd/applications"
	assert.NotEqual(t, duplicatedURL, redirectURL, "Redirect URL should not have duplicated rootPath")

	// The correct URL should be
	correctURL := "https://example.com:8080/argocd/applications"
	assert.Equal(t, correctURL, redirectURL, "Redirect URL should be correct without duplicated rootPath")
}
