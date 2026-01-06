package github_app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc adapts a plain function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// validAccessTokenBody returns a JSON access-token body with an expiry far
// enough in the future that isExpired() returns false.
func validAccessTokenBody(token string) []byte {
	at := accessToken{Token: token, ExpiresAt: time.Now().Add(time.Hour)}
	b, _ := json.Marshal(at)
	return b
}

// newIsolatedTransport creates a GitHubAppCacheTokenTransport with an isolated
// (non-global) tokenEntry so tests do not interfere with each other via
// the process-wide globalTokenRegistry.
func newIsolatedTransport(parent http.RoundTripper, baseURL string, _, _ int64) *GitHubAppCacheTokenTransport {
	return &GitHubAppCacheTokenTransport{
		parent:          parent,
		requestURLCache: baseURL + "/app/installations/42/access_tokens",
		tokenEntry:      &tokenEntry{},
	}
}

// setTokenEntry is a test helper that injects a token directly into the
// transport's tokenEntry, bypassing the need for a real upstream call.
func setTokenEntry(tr *GitHubAppCacheTokenTransport, token *accessToken) {
	tr.tokenEntry.mu.Lock()
	tr.tokenEntry.token = token
	tr.tokenEntry.mu.Unlock()
}

// getTokenEntry is a test helper that reads the current cached token.
func getTokenEntry(tr *GitHubAppCacheTokenTransport) *accessToken {
	tr.tokenEntry.mu.Lock()
	defer tr.tokenEntry.mu.Unlock()
	return tr.tokenEntry.token
}

// TestGetInstallationClient_TokenCacheEnabled verifies that when
// enableTokenCache is true the token is served from the process-wide registry but the upstream
// is called to verify that the app credentials are still valid.
func TestGetInstallationClient_TokenCacheEnabled(t *testing.T) {
	// Use unique IDs so this test's registry entry does not collide with others.
	const (
		testAppID          int64 = 9991
		testInstallationID int64 = 9992
	)
	// Pre-populate the global registry so the second newIsolatedTransport call
	// below can observe a shared state without a real upstream.  We simulate
	// the scenario where a previous reconciliation loop already fetched a token.
	key := tokenRegistryKey(testAppID, testInstallationID)
	state := &tokenEntry{
		token: &accessToken{Token: "tok-persisted", ExpiresAt: time.Now().Add(time.Hour)},
	}
	globalTokenRegistry.Store(key, state)
	t.Cleanup(func() { globalTokenRegistry.Delete(key) })

	// Build a transport that would go upstream — but the shared state is pre-populated.
	upstreamCalls := 0
	parent := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		upstreamCalls++
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(string(validAccessTokenBody(fmt.Sprintf("tok-upstream-%d", upstreamCalls))))),
		}, nil
	})

	tr := NewGitHubAppCacheTokenTransport(parent, testAppID, testInstallationID)
	tokenURL := "https://api.github.com/app/installations/9992/access_tokens"

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, tokenURL, http.NoBody)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, 1, upstreamCalls,
		"pre-populated shared state must be served while still hitting upstream")

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var at accessToken
	require.NoError(t, json.Unmarshal(body, &at))
	assert.Equal(t, "tok-persisted", at.Token,
		"response must contain the token from the shared registry, not the upstream token")
}

// TestCacheTokenTransport_SharedStateAcrossInstances verifies that two
// transport instances created with the same (appID, installationID, baseURL)
// share the same token state — simulating different reconciliation loops.
func TestCacheTokenTransport_SharedStateAcrossInstances(t *testing.T) {
	// Use unique IDs to isolate this test from the global registry.
	const (
		testAppID          int64 = 8881
		testInstallationID int64 = 8882
	)
	t.Cleanup(func() {
		globalTokenRegistry.Delete(tokenRegistryKey(testAppID, testInstallationID))
	})

	upstreamCalls := 0
	parent := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		upstreamCalls++
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(string(validAccessTokenBody(fmt.Sprintf("tok-shared-%d", upstreamCalls))))),
		}, nil
	})

	tokenURL := "https://api.github.com/app/installations/8882/access_tokens"

	// First transport (first reconciliation loop) fetches and caches the token.
	tr1 := NewGitHubAppCacheTokenTransport(parent, testAppID, testInstallationID)
	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, tokenURL, http.NoBody)
	resp1, err := tr1.RoundTrip(req1)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp1.Body)
	resp1.Body.Close()
	assert.Equal(t, 1, upstreamCalls, "first loop must hit upstream")

	// Second transport (second reconciliation loop) gets a new instance but
	// shares the same registry entry
	tr2 := NewGitHubAppCacheTokenTransport(parent, testAppID, testInstallationID)
	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, tokenURL, http.NoBody)
	resp2, err := tr2.RoundTrip(req2)
	require.NoError(t, err)

	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	var at accessToken
	require.NoError(t, json.Unmarshal(body2, &at))
	assert.Equal(t, "tok-shared-1", at.Token,
		"second loop must receive the token fetched by the first loop")
	assert.Equal(t, 2, upstreamCalls,
		"second loop while still hitting upstream")
}

// TestCacheTokenTransport_ExpiredTokenRefetchesFromUpstream verifies that once
// the cached token's refresh deadline the upstream token is fetched again and the cache is updated.
func TestCacheTokenTransport_ExpiredTokenRefetchesFromUpstream(t *testing.T) {
	callCount := 0
	parent := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(string(validAccessTokenBody("tok-fresh")))),
		}, nil
	})

	tr := newIsolatedTransport(parent, "https://api.github.com", 1, 42)
	// Inject an already-expired token.
	setTokenEntry(tr, &accessToken{Token: "tok-stale", ExpiresAt: time.Now().Add(-time.Hour)})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://api.github.com/app/installations/42/access_tokens", http.NoBody)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var at accessToken
	require.NoError(t, json.Unmarshal(body, &at))
	assert.Equal(t, "tok-fresh", at.Token,
		"response must contain the upstream token")

	cached := getTokenEntry(tr)
	require.NotNil(t, cached)
	assert.Equal(t, "tok-fresh", cached.Token, "cache must hold the refreshed token")
}

func ptr[T any](v T) *T { return &v }

func TestIsTokenPermissionsChanged_IdenticalPermissions(t *testing.T) {
	perm := github.InstallationPermissions{
		Contents: ptr("read"),
		Issues:   ptr("write"),
	}
	cached := &accessToken{Permissions: perm}
	incoming := &accessToken{Permissions: perm}
	assert.False(t, isTokenPermissionsChanged(cached, incoming),
		"identical permissions must not be reported as changed")
}

func TestIsTokenPermissionsChanged_PermissionLevelChanged(t *testing.T) {
	cached := &accessToken{
		Permissions: github.InstallationPermissions{Contents: ptr("read")},
	}
	incoming := &accessToken{
		Permissions: github.InstallationPermissions{Contents: ptr("write")},
	}
	assert.True(t, isTokenPermissionsChanged(cached, incoming),
		"permission level change (read→write) must be detected")
}

func TestIsTokenPermissionsChanged_PermissionAdded(t *testing.T) {
	cached := &accessToken{
		Permissions: github.InstallationPermissions{Contents: ptr("read")},
	}
	incoming := &accessToken{
		Permissions: github.InstallationPermissions{
			Contents: ptr("read"),
			Issues:   ptr("write"),
		},
	}
	assert.True(t, isTokenPermissionsChanged(cached, incoming),
		"addition of a new permission must be detected")
}

func TestIsTokenPermissionsChanged_PermissionRemoved(t *testing.T) {
	cached := &accessToken{
		Permissions: github.InstallationPermissions{
			Contents: ptr("read"),
			Issues:   ptr("write"),
		},
	}
	incoming := &accessToken{
		Permissions: github.InstallationPermissions{Contents: ptr("read")},
	}
	assert.True(t, isTokenPermissionsChanged(cached, incoming),
		"removal of a permission must be detected")
}

func TestIsTokenPermissionsChanged_NoRepositoryScope(t *testing.T) {
	// Empty Repositories on both sides means "all repos" — must not differ.
	cached := &accessToken{}
	incoming := &accessToken{}
	assert.False(t, isTokenPermissionsChanged(cached, incoming),
		"both tokens with empty repository scope must not be reported as changed")
}

func TestIsTokenPermissionsChanged_IdenticalRepositories(t *testing.T) {
	repos := []github.Repository{
		{ID: ptr(int64(1)), Name: ptr("repo-a")},
		{ID: ptr(int64(2)), Name: ptr("repo-b")},
	}
	cached := &accessToken{Repositories: repos}
	incoming := &accessToken{Repositories: repos}
	assert.False(t, isTokenPermissionsChanged(cached, incoming),
		"identical repository scopes must not be reported as changed")
}

func TestIsTokenPermissionsChanged_RepositoryAdded(t *testing.T) {
	cached := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
		},
	}
	incoming := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
			{ID: ptr(int64(2)), Name: ptr("repo-b")},
		},
	}
	assert.True(t, isTokenPermissionsChanged(cached, incoming),
		"addition of a repository must be detected")
}

func TestIsTokenPermissionsChanged_RepositoryRemoved(t *testing.T) {
	cached := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
			{ID: ptr(int64(2)), Name: ptr("repo-b")},
		},
	}
	incoming := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
		},
	}
	assert.True(t, isTokenPermissionsChanged(cached, incoming),
		"removal of a repository must be detected")
}

func TestIsTokenPermissionsChanged_RepositoryReordered(t *testing.T) {
	// The GitHub API may return repositories in a different order on a subsequent
	// call. A reorder must NOT be treated as a permission change.
	cached := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
			{ID: ptr(int64(2)), Name: ptr("repo-b")},
		},
	}
	incoming := &accessToken{
		Repositories: []github.Repository{
			{ID: ptr(int64(2)), Name: ptr("repo-b")},
			{ID: ptr(int64(1)), Name: ptr("repo-a")},
		},
	}
	assert.False(t, isTokenPermissionsChanged(cached, incoming),
		"reordered repository list with same IDs must not be reported as changed")
}
