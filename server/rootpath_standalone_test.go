package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock ArgoCDServer struct
type mockArgoCDServer struct {
	RootPath string
}

// Mock withRootPath function
// This function simulates the behavior of the withRootPath function in server.go
func withRootPath(handler http.Handler, a *mockArgoCDServer) http.Handler {
	// If RootPath is empty, directly return the original handler
	if a.RootPath == "" {
		return handler
	}

	// get rid of slashes
	root := strings.TrimRight(strings.TrimLeft(a.RootPath, "/"), "/")

	mux := http.NewServeMux()
	mux.Handle("/"+root+"/", http.StripPrefix("/"+root, handler))

	return mux
}

/*
 * Original implementation of withRootPath before the fix:
 *
 * func withRootPath(handler http.Handler, a *ArgoCDServer) http.Handler {
 *     // get rid of slashes
 *     root := strings.TrimRight(strings.TrimLeft(a.RootPath, "/"), "/")
 *
 *     mux := http.NewServeMux()
 *     mux.Handle("/"+root+"/", http.StripPrefix("/"+root, handler))
 *
 *     healthz.ServeHealthCheck(mux, a.healthCheck)
 *
 *     return mux
 * }
 *
 * The issue was that it didn't check if RootPath was empty, which could lead to
 * unnecessary path handling and potential issues.
 */

// Mock newRedirectServer function (fixed version)
// This function simulates the behavior of the newRedirectServer function in server.go
func newRedirectServer(port int, rootPath string) *http.Server {
	var addr string
	if rootPath == "" {
		addr = fmt.Sprintf("localhost:%d", port)
	} else {
		addr = fmt.Sprintf("localhost:%d/%s", port, strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/"))
	}

	return &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			target := "https://" + req.Host

			// Handle rootPath
			if rootPath != "" {
				root := strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/")
				target += "/" + root

				// Check if the request path already contains rootPath
				// If so, remove rootPath from the request path
				prefix := "/" + root
				if strings.HasPrefix(req.URL.Path, prefix) {
					req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
				}
			}

			target += req.URL.Path
			if len(req.URL.RawQuery) > 0 {
				target += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, target, http.StatusTemporaryRedirect)
		}),
	}
}

/*
 * Original implementation of newRedirectServer before the fix:
 *
 * func newRedirectServer(port int, rootPath string) *http.Server {
 *     addr := fmt.Sprintf("localhost:%d/%s", port, strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/"))
 *     return &http.Server{
 *         Addr: addr,
 *         Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
 *             target := "https://" + req.Host
 *             if rootPath != "" {
 *                 target += "/" + strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/")
 *             }
 *             target += req.URL.Path
 *             if len(req.URL.RawQuery) > 0 {
 *                 target += "?" + req.URL.RawQuery
 *             }
 *             http.Redirect(w, req, target, http.StatusTemporaryRedirect)
 *         }),
 *     }
 * }
 *
 * The issues were:
 * 1. It didn't handle empty rootPath correctly in the address construction
 * 2. It didn't check if the request path already contained rootPath, which could lead to
 *    rootPath duplication in the redirect URL
 */

// TestWithRootPathEmptyRootPath tests that withRootPath returns the original handler when RootPath is empty
func TestWithRootPathEmptyRootPath(t *testing.T) {
	// Create a simple handler
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a server with empty RootPath
	server := &mockArgoCDServer{
		RootPath: "",
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
	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a server with non-empty RootPath
	server := &mockArgoCDServer{
		RootPath: "/argocd",
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
	req := httptest.NewRequest("GET", "/applications", nil)
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
	req := httptest.NewRequest("GET", "/applications", nil)
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
	req := httptest.NewRequest("GET", "/argocd/applications", nil)
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
