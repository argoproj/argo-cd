package helm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexOCIChangedTagsAndAuthentication(t *testing.T) {
	var version, tagRequests, indexRequests atomic.Int32
	var authorized atomic.Bool
	version.Store(1)
	authorized.Store(true)
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "index.yaml") {
			indexRequests.Add(1)
		}
		username, password, ok := r.BasicAuth()
		if !authorized.Load() || !ok || username != "reader" || password != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/v2/child/tags/list" {
			tagRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "child", "tags": []string{fmt.Sprintf("%d.0.0", version.Load())}})
			return
		}
		if r.URL.Path != "/v2/" {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(registry.Close)
	cache := &fakeIndexCache{}
	client := NewClient(strings.TrimPrefix(registry.URL, "http://"), HelmCreds{Username: "reader", Password: "secret"}, true, "", "",
		WithPlainHTTP(), WithIndexCacheScopedByCreds(cache))
	tags, err := client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags)
	version.Store(2)
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags, "ordinary calls retain the existing tag-cache policy")
	require.EqualValues(t, 1, tagRequests.Load())
	tags, err = client.GetTags(t.Context(), "child", true)
	require.NoError(t, err)
	require.Equal(t, []string{"2.0.0"}, tags, "forced tag resolution must observe newly published versions")
	require.EqualValues(t, 2, tagRequests.Load())
	version.Store(3)
	cache.data = nil // Simulate tag-cache eviction/expiry.
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"3.0.0"}, tags)
	require.EqualValues(t, 3, tagRequests.Load())
	authorized.Store(false)
	_, err = client.GetTags(t.Context(), "child", true)
	require.ErrorContains(t, err, "401", "a warmed tag cache must not mask forced authentication failures")
	require.Zero(t, indexRequests.Load(), "OCI tag discovery must never request index.yaml")
}
