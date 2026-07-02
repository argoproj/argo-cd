package github_app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/google/go-github/v69/github"
	log "github.com/sirupsen/logrus"
)

// accessToken mirrors the GitHub API response for
// POST /app/installations/{id}/access_tokens.
type accessToken struct {
	Token        string                         `json:"token"`
	ExpiresAt    time.Time                      `json:"expires_at"`
	Permissions  github.InstallationPermissions `json:"permissions,omitempty"`
	Repositories []github.Repository            `json:"repositories,omitempty"`
}

// refreshTime is how far before ExpiresAt we treat the token as stale.
// GitHub issues tokens valid for 1 h; refreshing 1 min early avoids
// accepting a token that would expire mid-request.
const refreshLeadTime = time.Minute

func (at *accessToken) isExpired() bool {
	if at == nil {
		return true
	}
	return at.ExpiresAt.Add(-refreshLeadTime).Before(time.Now())
}

type tokenEntry struct {
	mu    sync.Mutex
	token *accessToken
}

// tokenRegistryKey returns a unique string key for the given GitHub App
// installation endpoint. It is used as the key in globalTokenRegistry so the
// linter can see that every component is actively used in key construction.
func tokenRegistryKey(appID, installationID int64) string {
	return fmt.Sprintf("%d/%d", appID, installationID)
}

var globalTokenRegistry sync.Map

// newTokenEntry returns the existing tokenEntry for the
// given key, or atomically creates and stores a new one.
func newTokenEntry(key string) *tokenEntry {
	val, _ := globalTokenRegistry.LoadOrStore(key, &tokenEntry{})
	return val.(*tokenEntry)
}

type GitHubAppCacheTokenTransport struct {
	parent          http.RoundTripper
	requestURLCache string
	tokenEntry      *tokenEntry
}

func (t *GitHubAppCacheTokenTransport) needInterception(req *http.Request) bool {
	return req.Method == http.MethodPost && req.URL.String() == t.requestURLCache
}

// isTokenPermissionsChanged reports whether the upstream token represents a
// different permission or repository scope than the cached token.
//
// Two dimensions are compared:
//
//   - Permissions: the set of GitHub API permission levels granted to the token
//     (e.g. contents:read, issues:write). Any addition, removal, or level change
//     requires the cached token to be replaced.
//
//   - Repositories: the explicit list of repositories the token is scoped to.
//     An empty slice means "all repositories the installation has access to".
//     A non-empty slice means the token is restricted to those specific repos.
//     Any change in the set (order-independent) requires replacement.
//
// reflect.DeepEqual is used deliberately:
//   - InstallationPermissions has 60+ *string fields. Manual field-by-field
//     comparison would need updating every time the GitHub API adds a permission.
//   - Repository is a deeply nested struct. DeepEqual handles pointer
//     indirection and nested values correctly for values decoded from JSON.
//   - Both structs were freshly decoded from JSON so there are no cyclic
//     references or unexported state that could confuse DeepEqual.
func isTokenPermissionsChanged(cached, incoming *accessToken) bool {
	if cached == nil || incoming == nil {
		return cached != incoming
	}

	// Compare permission scopes.
	if !reflect.DeepEqual(cached.Permissions, incoming.Permissions) {
		return true
	}

	// Compare repository scopes. The GitHub API returns repositories in a
	// stable order for a given installation, but we sort by ID for robustness
	// so that a reordered response does not trigger an unnecessary cache miss.
	if len(cached.Repositories) != len(incoming.Repositories) {
		return true
	}
	// Build ID-keyed sets from each slice and compare them.
	// Repository.ID is the canonical stable identifier.
	cachedIDs := make(map[int64]struct{}, len(cached.Repositories))
	for _, r := range cached.Repositories {
		if r.ID != nil {
			cachedIDs[*r.ID] = struct{}{}
		}
	}
	for _, r := range incoming.Repositories {
		if r.ID == nil {
			continue
		}
		if _, ok := cachedIDs[*r.ID]; !ok {
			return true
		}
	}

	return false
}

func (t *GitHubAppCacheTokenTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if !t.needInterception(req) {
		return t.parent.RoundTrip(req)
	}

	// Always perform the upstream request to check that application permissions have not changed
	resp, err = t.parent.RoundTrip(req)

	// If the request was unsuccessful, return the response and error
	if err != nil || resp.StatusCode/100 != 2 {
		return resp, err
	}

	// check if the response is a valid access token response
	bodyBytes, errRead := io.ReadAll(resp.Body)
	resp.Body.Close()
	// Restore body so the caller can still read it.
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	// - If the response body is empty, return the original response and error
	if errRead != nil {
		return resp, err
	}

	newToken := &accessToken{}
	// - If the response body is not valid JSON, return the original response and error
	if errJson := json.Unmarshal(bodyBytes, newToken); errJson != nil {
		return resp, err
	}
	// - If the new token is expired, return the original response and error
	if newToken.isExpired() {
		return resp, err
	}

	t.tokenEntry.mu.Lock()
	defer t.tokenEntry.mu.Unlock()
	// If the cached token is expired or the permissions have changed, update the cache
	if t.tokenEntry.token.isExpired() || isTokenPermissionsChanged(t.tokenEntry.token, newToken) {
		t.tokenEntry.token = newToken
		// Return the original response and error
		return resp, err
	}

	// If the cached token is not expired, return the cached token in the response
	cachedTokenBody, errJson := json.Marshal(t.tokenEntry.token)
	if errJson != nil {
		return resp, err
	}
	// Drain and close original body to allow connection reuse; the error is
	// intentionally ignored here — failure to drain does not affect correctness,
	// and the body is replaced immediately after.
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	newBody := io.NopCloser(bytes.NewReader(cachedTokenBody))
	resp.Body = newBody
	resp.ContentLength = int64(len(cachedTokenBody))
	return resp, err
}

func NewGitHubAppCacheTokenTransport(parent http.RoundTripper, appID, installationID int64) *GitHubAppCacheTokenTransport {
	log.Debug("Creating new GitHub App token cache transport")
	key := tokenRegistryKey(appID, installationID)
	return &GitHubAppCacheTokenTransport{
		parent:          parent,
		requestURLCache: fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID),
		tokenEntry:      newTokenEntry(key),
	}
}
