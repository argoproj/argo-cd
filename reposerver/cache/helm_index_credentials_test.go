package cache

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/helm"
)

type operationCredentials struct {
	helm.HelmCreds
	mu     sync.Mutex
	tokens []string
	calls  int
	err    error
}

func (c *operationCredentials) GetPassword() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.tokens[min(c.calls-1, len(c.tokens)-1)], nil
}

func credentialCacheKey(url, password string) string {
	return url + "\x00" + helm.CredsScope(helm.HelmCreds{Username: "reader", Password: password})
}

func TestHelmIndexCredentialsStayConsistentAcrossOperations(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	var requestLock sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, _ := r.BasicAuth()
		requestLock.Lock()
		requests = append(requests, password)
		requestLock.Unlock()
		version := "1.0.0"
		if password == "token-B" {
			version = "2.0.0"
		}
		w.Header().Set("ETag", `"`+version+`"`)
		_, _ = w.Write(helmIndexContractBody(version))
	}))
	t.Cleanup(server.Close)
	provider := &operationCredentials{
		Username: "reader",
		tokens:   []string{"token-A", "token-B"},
	}
	client := helm.NewClient(server.URL, provider, false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	for i, version := range []string{"1.0.0", "2.0.0", "2.0.0"} {
		index, err := client.GetIndex(t.Context(), i == 0, 1<<20)
		require.NoError(t, err)
		require.Equal(t, version, index.Entries["child"][0].Version)
	}
	requestLock.Lock()
	gotRequests := append([]string(nil), requests...)
	requestLock.Unlock()
	assert.Equal(t, []string{"token-A", "token-B"}, gotRequests, "the second token must fetch its own view; the third operation should hit that cache")
	assert.Equal(t, 3, provider.calls, "resolve the provider exactly once for every operation, including cache hits")
	for password, version := range map[string]string{"token-A": "1.0.0", "token-B": "2.0.0"} {
		record, err := cache.GetHelmIndexRecord(credentialCacheKey(server.URL, password))
		require.NoError(t, err)
		assert.Equal(t, helmIndexContractBody(version), record.Body)
		assert.Equal(t, `"`+version+`"`, record.ETag)
	}
}

func TestHelmIndexCredentialFailureCannotUseWarmCache(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	url := "https://example.invalid"
	require.NoError(t, cache.SetHelmIndexRecord(credentialCacheKey(url, "token-A"), helmIndexContractBody("1.0.0"), `"one"`, ""))
	provider := &operationCredentials{Username: "reader", tokens: []string{"token-A"}}
	client := helm.NewClient(url, provider, false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	_, err := client.GetIndex(t.Context(), false, 1<<20)
	require.NoError(t, err)
	provider.err = errors.New("credential lookup failed")
	_, err = client.GetIndex(t.Context(), false, 1<<20)
	require.ErrorContains(t, err, "credential lookup failed", "a previous operation's warm entry cannot bypass current credential resolution")
}

func TestHelmTagsCacheUsesCurrentCredentialView(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	url := "registry.example.invalid/child"
	require.NoError(t, cache.SetHelmIndex(credentialCacheKey(url, "token-A"), []byte(`{"Tags":["1.0.0"]}`)))
	require.NoError(t, cache.SetHelmIndex(credentialCacheKey(url, "token-B"), []byte(`{"Tags":["2.0.0"]}`)))
	provider := &operationCredentials{Username: "reader", tokens: []string{"token-A", "token-B"}}
	client := helm.NewClient("registry.example.invalid", provider, true, "", "", helm.WithIndexCacheScopedByCreds(cache))
	for _, version := range []string{"1.0.0", "2.0.0"} {
		tags, err := client.GetTags(t.Context(), "child", false)
		require.NoError(t, err)
		assert.Equal(t, []string{version}, tags)
	}
	assert.Equal(t, 2, provider.calls)
}
