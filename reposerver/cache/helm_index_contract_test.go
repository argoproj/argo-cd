package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

// This fixture implements actual conditional responses. A server that always
// returns 200 cannot detect a mismatched body/ETag or prove retention saves bytes.
type helmIndexContractOrigin struct {
	mu       sync.Mutex
	version  string
	statuses []int
	bytes    []int
}

func helmIndexContractBody(version string) []byte {
	return []byte(fmt.Sprintf("apiVersion: v1\nentries:\n  child:\n    - version: %s\n", version))
}

func (o *helmIndexContractOrigin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	etag := `"` + o.version + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		o.statuses = append(o.statuses, http.StatusNotModified)
		o.bytes = append(o.bytes, 0)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := helmIndexContractBody(o.version)
	o.statuses = append(o.statuses, http.StatusOK)
	o.bytes = append(o.bytes, len(body))
	_, _ = w.Write(body)
}

func helmIndexContractSetup(t *testing.T) (*Cache, *miniredis.Miniredis, *helmIndexContractOrigin, string) {
	t.Helper()
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { require.NoError(t, redisClient.Close()) })
	base := cacheutil.NewCache(cacheutil.NewRedisCache(redisClient, time.Hour, cacheutil.RedisCompressionNone))
	cache := NewCache(base, time.Hour, time.Minute, 10*time.Second)
	origin := &helmIndexContractOrigin{version: "1.0.0"}
	server := httptest.NewServer(origin)
	t.Cleanup(server.Close)
	return cache, redisServer, origin, server.URL
}

func TestHelmIndexRedisContractRetainsBodyAndValidators(t *testing.T) {
	cache, redisServer, origin, url := helmIndexContractSetup(t)
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	redisServer.FastForward(2 * time.Minute)
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", index.Entries["child"][0].Version)
	origin.mu.Lock()
	defer origin.mu.Unlock()
	require.Equal(t, []int{http.StatusOK, http.StatusNotModified}, origin.statuses,
		"after freshness expires, retained body and validators must permit 304")
	assert.Zero(t, origin.bytes[1])
}

func TestHelmIndexRedisContractLegacyWriterCannotPoison304(t *testing.T) {
	cache, _, origin, url := helmIndexContractSetup(t)
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	origin.mu.Lock()
	origin.version = "2.0.0"
	origin.mu.Unlock()
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", index.Entries["child"][0].Version)
	// Simulate a delayed old replica: it only knows the legacy raw-body key.
	// It must not be able to change the new representation associated with v2.
	require.NoError(t, cache.SetHelmIndex(url, helmIndexContractBody("1.0.0")))
	index, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version,
		"a matching origin ETag must never authorize an old replica's body")
}
