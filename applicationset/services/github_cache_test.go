package services

import (
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh_hash_token "github.com/bored-engineer/github-conditional-http-transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

func TestCacheable(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "GET is cacheable",
			method:   http.MethodGet,
			path:     "/repos/org/repo/branches",
			expected: true,
		},
		{
			name:     "HEAD is cacheable",
			method:   http.MethodHead,
			path:     "/repos/org/repo/branches",
			expected: true,
		},
		{
			name:     "POST is not cacheable",
			method:   http.MethodPost,
			path:     "/repos/org/repo/branches",
			expected: false,
		},
		{
			name:     "PUT is not cacheable",
			method:   http.MethodPut,
			path:     "/repos/org/repo",
			expected: false,
		},
		{
			name:     "DELETE is not cacheable",
			method:   http.MethodDelete,
			path:     "/repos/org/repo",
			expected: false,
		},
		{
			name:     "PATCH is not cacheable",
			method:   http.MethodPatch,
			path:     "/repos/org/repo",
			expected: false,
		},
		{
			name:     "GET with Range header is not cacheable",
			method:   http.MethodGet,
			path:     "/repos/org/repo/branches",
			headers:  map[string]string{"Range": "bytes=0-100"},
			expected: false,
		},
		{
			name:     "GET /rate_limit is not cacheable",
			method:   http.MethodGet,
			path:     "/rate_limit",
			expected: false,
		},
		{
			name:     "GET /api/v3/rate_limit is not cacheable (GitHub Enterprise)",
			method:   http.MethodGet,
			path:     "/api/v3/rate_limit",
			expected: false,
		},
		{
			name:     "GET other path is cacheable",
			method:   http.MethodGet,
			path:     "/api/v3/repos/org/repo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tt.method, "https://api.github.com"+tt.path, http.NoBody)
			require.NoError(t, err)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, cacheable(req))
		})
	}
}

func TestParseVaryHeaders(t *testing.T) {
	tests := []struct {
		name        string
		varyValues  []string
		expected    []string
		expectError bool
	}{
		{
			name:       "empty Vary header",
			varyValues: []string{},
			expected:   []string{},
		},
		{
			name:       "single field",
			varyValues: []string{"Accept"},
			expected:   []string{"Accept"},
		},
		{
			name:       "multiple fields in one header",
			varyValues: []string{"Accept, Authorization"},
			expected:   []string{"Accept", "Authorization"},
		},
		{
			name:       "multiple separate Vary headers",
			varyValues: []string{"Accept", "Authorization"},
			expected:   []string{"Accept", "Authorization"},
		},
		{
			name:        "wildcard Vary header returns error",
			varyValues:  []string{"*"},
			expected:    []string{},
			expectError: true,
		},
		{
			name:       "canonical header key normalisation",
			varyValues: []string{"accept-encoding"},
			expected:   []string{"Accept-Encoding"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			for _, v := range tt.varyValues {
				headers.Add("Vary", v)
			}
			result, err := parseVaryHeaders(headers)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsSameVaryHeader(t *testing.T) {
	tokenOK := "token secret123"
	hashedToken := gh_hash_token.HashToken(tokenOK)

	tests := []struct {
		name             string
		reqHeaders       map[string]string
		varyHeaders      []string
		varyHeadersValue map[string]string
		expected         bool
	}{
		{
			name:             "no vary headers always matches",
			reqHeaders:       map[string]string{},
			varyHeaders:      []string{},
			varyHeadersValue: map[string]string{},
			expected:         true,
		},
		{
			name:             "Authorization matched by hashed token",
			reqHeaders:       map[string]string{"Authorization": tokenOK},
			varyHeaders:      []string{"Authorization"},
			varyHeadersValue: map[string]string{"Authorization": hashedToken},
			expected:         true,
		},
		{
			name:             "Authorization mismatched token",
			reqHeaders:       map[string]string{"Authorization": "token other"},
			varyHeaders:      []string{"Authorization"},
			varyHeadersValue: map[string]string{"Authorization": hashedToken},
			expected:         false,
		},
		{
			name:             "non-Authorization header matches",
			reqHeaders:       map[string]string{"Accept": "application/json"},
			varyHeaders:      []string{"Accept"},
			varyHeadersValue: map[string]string{"Accept": "application/json"},
			expected:         true,
		},
		{
			name:             "non-Authorization header mismatches",
			reqHeaders:       map[string]string{"Accept": "text/html"},
			varyHeaders:      []string{"Accept"},
			varyHeadersValue: map[string]string{"Accept": "application/json"},
			expected:         false,
		},
		{
			name: "all vary headers must match",
			reqHeaders: map[string]string{
				"Accept":        "application/json",
				"Authorization": tokenOK,
			},
			varyHeaders:      []string{"Accept", "Authorization"},
			varyHeadersValue: map[string]string{"Accept": "application/json", "Authorization": hashedToken},
			expected:         true,
		},
		{
			name: "one vary header mismatches fails overall",
			reqHeaders: map[string]string{
				"Accept":        "text/html",
				"Authorization": tokenOK,
			},
			varyHeaders:      []string{"Accept", "Authorization"},
			varyHeadersValue: map[string]string{"Accept": "application/json", "Authorization": hashedToken},
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.github.com/repos", http.NoBody)
			require.NoError(t, err)
			for k, v := range tt.reqHeaders {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, isSameVaryHeader(req, tt.varyHeaders, tt.varyHeadersValue))
		})
	}
}

func TestNewLRUStorage_CacheKeys(t *testing.T) {
	tests := []struct {
		name        string
		cacheCtx    *GitHubCacheContext
		expectedKey string
	}{
		{
			name:        "anonymous when no auth provided",
			cacheCtx:    &GitHubCacheContext{},
			expectedKey: "anonymous",
		},
		{
			name:        "app-secret key from AppSecretName",
			cacheCtx:    &GitHubCacheContext{AppSecretName: "my-app-secret"},
			expectedKey: "app/my-app-secret",
		},
		{
			name: "token key from TokenRef",
			cacheCtx: &GitHubCacheContext{
				TokenRef: &argoprojiov1alpha1.SecretRef{SecretName: "my-secret", Key: "token"},
			},
			expectedKey: "token/my-secret/token",
		},
		{
			name: "AppSecretName takes precedence over TokenRef",
			cacheCtx: &GitHubCacheContext{
				AppSecretName: "app-secret",
				TokenRef:      &argoprojiov1alpha1.SecretRef{SecretName: "my-secret", Key: "token"},
			},
			expectedKey: "app/app-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newLRUStorage(tt.cacheCtx, 10)
			assert.Equal(t, tt.expectedKey, storage.key)
		})
	}
}

func TestNewLRUStorage_ReusesSameStorage(t *testing.T) {
	cacheCtx := &GitHubCacheContext{AppSecretName: "reuse-test-secret"}
	s1 := newLRUStorage(cacheCtx, 10)
	s2 := newLRUStorage(cacheCtx, 10)
	// Same pointer to underlying lruMap means the same storage is returned
	assert.Same(t, s1.lock, s2.lock, "Same storage instance should be reused for identical context")
}

func TestGitHubCache_NoEtagResponseNotCached(t *testing.T) {
	// Responses without an ETag should not be stored, so every request hits upstream
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	cacheCtx := &GitHubCacheContext{AppSecretName: "no-etag-test"}
	client := &http.Client{Transport: NewGitHubCacheTransport(cacheCtx, 100, nil)}
	ctx := t.Context()

	for range 2 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/repos/org/repo", http.NoBody)
		require.NoError(t, err)
		res, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	}
	assert.Equal(t, 2, requestCount, "Responses without ETag must not be cached")
}

func TestGitHubCache_DifferentTokenIsolation(t *testing.T) {
	// Cache entries must not bleed across different authorization tokens
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "token ok") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		etag := "etag-" + auth
		ifNoneMatch := r.Header.Get("If-None-Match")
		if ifNoneMatch == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Etag", etag)
		w.Header().Set("Vary", "Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token": "` + auth + `"}`))
	}))
	defer ts.Close()

	cacheCtx := &GitHubCacheContext{AppSecretName: "isolation-test"}
	client := &http.Client{Transport: NewGitHubCacheTransport(cacheCtx, 100, nil)}
	ctx := t.Context()

	// Warm cache with token A
	reqA, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/repos/org/repo", http.NoBody)
	reqA.Header.Set("Authorization", "token ok-A")
	resA, err := client.Do(reqA)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resA.StatusCode)
	_, _ = io.ReadAll(resA.Body)
	resA.Body.Close()

	countAfterFirst := requestCount

	// Request with different token B should go upstream (not served from token A cache)
	reqB, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/repos/org/repo", http.NoBody)
	reqB.Header.Set("Authorization", "token ok-B")
	resB, err := client.Do(reqB)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resB.StatusCode)
	_, _ = io.ReadAll(resB.Body)
	resB.Body.Close()

	assert.Equal(t, countAfterFirst+1, requestCount, "Request with different token should not be served from cache")
}

func TestGitHubCache_ExcludedHeadersNotCached(t *testing.T) {
	// Volatile headers like X-RateLimit-* and X-GitHub-Request-ID must not be
	// restored from the cache when a 304 is received.  Only headers that are
	// absent from the live 304 response are merged in from the cache, so
	// headers present in the 304 itself (e.g. Date, set by the Go http server)
	// are allowed to appear in the final response.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "token ok") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ifNoneMatch := r.Header.Get("If-None-Match")
		if ifNoneMatch == `"stable-etag"` {
			// 304 response – do NOT echo back the volatile headers
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Etag", `"stable-etag"`)
		w.Header().Set("X-RateLimit-Remaining", "42")
		w.Header().Set("X-GitHub-Request-Id", "req-id-123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	cacheCtx := &GitHubCacheContext{AppSecretName: "excluded-headers-test"}
	client := &http.Client{Transport: NewGitHubCacheTransport(cacheCtx, 100, nil)}
	ctx := t.Context()

	// First request – response gets cached (volatile headers stripped before storing)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/repos/org/repo", http.NoBody)
	req.Header.Set("Authorization", "token ok1")
	res, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	_, _ = io.ReadAll(res.Body)
	res.Body.Close()

	// Second request – 304 from upstream; cache merges back stored headers.
	// Volatile headers (X-RateLimit-*, X-GitHub-Request-ID) must not be injected
	// from the cache into the response the caller sees.
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/repos/org/repo", http.NoBody)
	req2.Header.Set("Authorization", "token ok1")
	res2, err := client.Do(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res2.StatusCode)

	// These headers were present in the first response but must have been
	// stripped before storage, so they must not reappear in the cached response.
	volatileHeaders := []string{
		"X-RateLimit-Remaining",
		"X-GitHub-Request-Id",
	}
	for _, h := range volatileHeaders {
		assert.Empty(t, res2.Header.Get(h), "Volatile header %q must not be restored from cache", h)
	}
	_, _ = io.ReadAll(res2.Body)
	res2.Body.Close()
}

func TestGitHubCacheContext_TokenRefKey(t *testing.T) {
	// Verify that two different token refs produce distinct storage buckets
	ctxA := &GitHubCacheContext{
		TokenRef: &argoprojiov1alpha1.SecretRef{SecretName: "secret-a", Key: "token"},
	}
	ctxB := &GitHubCacheContext{
		TokenRef: &argoprojiov1alpha1.SecretRef{SecretName: "secret-b", Key: "token"},
	}
	storageA := newLRUStorage(ctxA, 10)
	storageB := newLRUStorage(ctxB, 10)
	assert.NotEqual(t, storageA.key, storageB.key, "Different TokenRefs must produce different storage keys")
}
