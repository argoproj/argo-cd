package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// These tests came out of the code review of the Helm index cache (issue
// #30949): they assert the Redis layer's externally relevant guarantees and
// deliberately fail on the reviewed regressions. They live in this package
// because util/helm cannot import reposerver/cache (import cycle).

func regressionRedisCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	base := cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone))
	return NewCache(base, time.Hour, time.Minute, 10*time.Second), server
}

// regressionChartRepo serves a minimal helm repo whose index can be
// re-published with a new chart version between requests.
type regressionChartRepo struct {
	server *httptest.Server
	mu     sync.Mutex
	index  string
}

func (f *regressionChartRepo) publish(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.index = fmt.Sprintf(`apiVersion: v1
entries:
  child:
    - name: child
      version: %s
      urls:
        - %s/child-%s.tgz
`, version, f.server.URL, version)
}

func newRegressionChartRepo(t *testing.T) *regressionChartRepo {
	t.Helper()
	f := &regressionChartRepo{}
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		index := f.index
		f.mu.Unlock()
		// Version-derived ETag: strong validator that changes when the
		// index is re-published, enabling 304 revalidation in tests.
		etag := `"` + f.indexVersion(t) + `"`
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(index))
	})
	f.server = httptest.NewServer(mux)
	f.publish("1.0.0")
	t.Cleanup(f.server.Close)
	return f
}

func TestIndexCacheRegressionRedisRetainsRevalidationDataAfterFreshness(t *testing.T) {
	cache, redisServer := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	// Advance Redis time without sleeping. The body's freshness expires, but
	// validators must be retained longer so the next read can revalidate
	// conditionally instead of re-downloading.
	redisServer.FastForward(2 * time.Minute)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
}

func TestIndexCacheRegressionLegacyRedisWriterCannotPoison304(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	var oldBody []byte
	require.NoError(t, cache.GetHelmIndex(fixture.server.URL, &oldBody))
	fixture.publish("2.0.0")
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", fixture.indexVersion(t), "origin now serves 2.0.0")
	// A delayed old replica writes only the legacy body, without the new
	// validator protocol. New records must be isolated from that legacy key:
	// a 304 must never pair a new validator with the old body.
	require.NoError(t, cache.SetHelmIndex(fixture.server.URL, oldBody))
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version, "an old replica must not make the client serve an old body under a new validator")
}

// indexVersion returns the version currently published by the fixture.
func (f *regressionChartRepo) indexVersion(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for line := range strings.SplitSeq(f.index, "\n") {
		if after, ok := strings.CutPrefix(line, "      version: "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// --- Round-2 review regressions (findings 2/3/4/6 of the second review) ---

// Finding 4: redis keys carry only the repo URL, so a private-credentialed
// connectivity check would populate cache entries that an anonymous reader
// consumes. With scoped clients, the two views must see disjoint keys.
func TestIndexCacheRegressionRedisKeysScopedByCredentials(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)

	private := helm.NewClient(fixture.server.URL,
		helm.HelmCreds{Username: "reader", Password: "secret"},
		false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	_, err := private.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// The private fetch publishes under a scoped key; the legacy bare-URL
	// keys must remain untouched (that is what an anonymous reader uses).
	var body []byte
	require.ErrorIs(t, cache.GetHelmIndex(fixture.server.URL, &body), cacheutil.ErrCacheMiss,
		"a private-credentialed fetch must not populate the shared bare-URL cache entry")
	_, _, err = cache.GetHelmIndexValidators(fixture.server.URL)
	require.Error(t, err, "validators must also stay scoped: an anonymous reader must not inherit private-fetch metadata")

	// An anonymous client with the SAME empty-credential scope shares keys
	// with other anonymous clients (that is the point: same view = same key).
	anon := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err = anon.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
}

// Finding 2: the v2 record must stop serving once body freshness expires
// (age > revision-cache TTL), even though the record itself is retained
// longer for 304 revalidation. A stale body served to an ordinary reader
// would extend staleness from minutes to the repo-cache window.
func TestIndexCacheRegressionRedisStaleBodyNotServedAfterFreshness(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// The client publishes under the SCOPED key — assert on the populated
	// key, not the bare URL (a bare-URL miss would pass regardless of
	// freshness and prove nothing).
	scopedKey := fixture.server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})

	// Freshness is measured from the record's StoredAt with time.Now() —
	// miniredis FastForward does NOT move that clock. Backdate the stored
	// validation timestamp to simulate real aging instead.
	rec, err := cache.GetHelmIndexRecord(scopedKey)
	require.NoError(t, err)
	require.NotEmpty(t, rec.Body, "the test must exercise the populated key")
	// Backdate the validation timestamp in place (package-internal SetItem).
	rec.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	require.NoError(t, cache.cache.SetItem(helmIndexV2Key(scopedKey), rec,
		&cacheutil.CacheActionOpts{Expiration: cache.repoCacheExpiration}))

	// Past freshness: ordinary readers must NOT get the stale body...
	var body []byte
	err = cache.GetHelmIndex(scopedKey, &body)
	require.ErrorIs(t, err, cacheutil.ErrCacheMiss, "stale body must not be served to ordinary readers after freshness expiry")

	// ...while the revalidation path can still read the retained record.
	retained, err := cache.GetHelmIndexRetainedRecord(scopedKey)
	require.NoError(t, err, "the retained record must survive freshness expiry for 304 revalidation")
	assert.NotEmpty(t, retained.Body)
	assert.Equal(t, rec.ETag, retained.ETag)
}

// Finding 3: body and validators must be read as ONE record during
// revalidation. Simulate a concurrent replica by rewriting the v2 record
// with a new body BETWEEN the client's record read and its publish; the
// conditional-GET path must never pair validator B with body A.
func TestIndexCacheRegressionConcurrentRecordReadAtomicity(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// Publish a different body+validator set, as a concurrent replica would.
	fixture.publish("2.0.0")
	scopedKey := fixture.server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})
	var body2 []byte
	require.NoError(t, cache.GetHelmIndex(scopedKey, &body2))
	rec, err := cache.GetHelmIndexRecord(scopedKey)
	require.NoError(t, err)

	// The client's revalidation (GetIndex with a retained record) must send
	// the validator that belongs to the body it read — one atomic record —
	// not a validator read separately from a record another replica replaced.
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version)
	assert.NotEmpty(t, rec.ETag, "the record must carry the validator with the body")
}

// Finding 6: a fresh client instance must keep validators across a 304 so
// the chain is 200 → 304 → 304 (not 200 → 304 → 200). Fresh clients arise
// naturally: repo-server builds a new client per request.
func TestIndexCacheRegressionFreshClientKeepsValidatorsAfter304(t *testing.T) {
	cache, redisServer := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)

	// Client 1: cold fetch, publishes body+validators (200).
	first := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err := first.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// Expire freshness so the next read revalidates conditionally.
	redisServer.FastForward(2 * time.Minute)

	// Client 2: fresh instance, sees expired record → conditional GET → 304.
	second := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err = second.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	scopedKey := fixture.server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})
	rec, err := cache.GetHelmIndexRecord(scopedKey)
	require.NoError(t, err)
	require.NotEmpty(t, rec.ETag, "a 304 on a fresh client must retain the validators, not republish empty ones")

	// Client 3: another fresh instance AFTER another expiry must again get a
	// 304 (validators survived), proving the chain 200 → 304 → 304.
	redisServer.FastForward(2 * time.Minute)
	third := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err = third.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
}
