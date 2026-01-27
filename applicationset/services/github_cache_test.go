package services

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh_hash_token "github.com/bored-engineer/github-conditional-http-transport"
	"github.com/stretchr/testify/assert"
)

var varyHeadersTest = []string{
	"Accept-Encoding",
	"Accept",
	"Authorization",
}

func setupFakeGithubServer(cacheHitCounter *int) *httptest.Server {
	payload := []byte(`{"name": "main"}`)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond on 200/304 only if Authorization header start with `token ok`
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "token ok") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// If the request contains If-None-Match header, respond with 304
		ifNoneMatch := r.Header.Get("If-None-Match")
		if ifNoneMatch != "" {
			*cacheHitCounter++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		// Otherwise respond with 200 with Etag and payload
		h := gh_hash_token.Hash(r.Header, varyHeadersTest)
		h.Write(payload)
		w.Header().Set("Etag", hex.EncodeToString(h.Sum(nil)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
}

func TestGitHubCache_Success(t *testing.T) {
	// Setup a fake HTTP server
	cacheHitCounter := 0
	ts := setupFakeGithubServer(&cacheHitCounter)
	defer ts.Close()

	cacheCtx := &GitHubCacheContext{
		AppSecretName: "app-secret-name",
	}
	client := &http.Client{
		Transport: NewGitHubCacheTransport(cacheCtx, 100, nil),
	}
	ctx := t.Context()

	// First request, should hit the server
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/owner/repo/branches/main", http.NoBody)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "token ok1")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected ok status")

	// Second request, should be served from cache with 304 if same Authorization token
	req.Header.Set("Authorization", "token ok1")
	res, err = client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected ok status")
	assert.Equal(t, 1, cacheHitCounter, "Expected one cache hit")
}

func TestGitHubCache_NotAuthorized(t *testing.T) {
	// Setup a fake HTTP server
	cacheHitCounter := 0
	ts := setupFakeGithubServer(&cacheHitCounter)
	defer ts.Close()

	cacheCtx := &GitHubCacheContext{
		AppSecretName: "app-secret-name",
	}
	client := &http.Client{
		Transport: NewGitHubCacheTransport(cacheCtx, 100, nil),
	}
	ctx := t.Context()

	// First request, should hit the server
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/owner/repo/branches/main", http.NoBody)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "token ok1")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected ok status")

	// Second request, should not be served from cache if Authorization is an invalid token
	req.Header.Set("Authorization", "token ko")
	res, err = client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "Expected unauthorized status")
	assert.Equal(t, 0, cacheHitCounter, "Expected no cache hit")
}

func TestNewGitHubCache(t *testing.T) {
	// Test cases
	testCases := []struct {
		name   string
		parent http.RoundTripper
	}{
		{
			name:   "with parent transport",
			parent: http.DefaultTransport,
		},
		{
			name:   "with nil parent transport",
			parent: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create client
			cacheCtx := &GitHubCacheContext{
				AppSecretName: "app-secret-name",
			}
			client := NewGitHubCache(cacheCtx, 100, tc.parent)

			// Assert client is not nil
			assert.NotNil(t, client)

			// Assert transport is properly configured
			_, ok := client.Transport.(*GitHubCacheTransport)
			assert.True(t, ok, "Transport should be GitHubCacheTransport")
		})
	}
}
