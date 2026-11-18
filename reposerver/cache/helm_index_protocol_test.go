package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

// Delays surround real Redis operations so publication, key encoding and
// ownership checks still run through the production backend.
type indexProtocolBackend struct {
	cacheutil.CacheClient
	stallMetadata bool
	stallBody     bool
	bodyDelay     time.Duration
	missDelay     time.Duration
	bodyReads     int
	stalls        int
}

func (b *indexProtocolBackend) GetContext(ctx context.Context, key string, obj any) error {
	delay := b.missDelay
	if strings.Contains(key, "|body") {
		b.bodyReads++
		delay = b.bodyDelay
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return b.CacheClient.(interface {
		GetContext(context.Context, string, any) error
	}).GetContext(ctx, key, obj)
}

func (b *indexProtocolBackend) BeginUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	return b.CacheClient.(interface {
		BeginUpdate(context.Context, string, string, time.Duration) (bool, error)
	}).BeginUpdate(ctx, key, token, expiration)
}

func (b *indexProtocolBackend) StoreVersioned(ctx context.Context, item cacheutil.VersionedItem) (bool, error) {
	if b.stallMetadata {
		b.stalls++
		<-ctx.Done()
		return false, ctx.Err()
	}
	return b.CacheClient.(interface {
		StoreVersioned(context.Context, cacheutil.VersionedItem) (bool, error)
	}).StoreVersioned(ctx, item)
}

func (b *indexProtocolBackend) StoreVersionedBody(ctx context.Context, referenceKey, bodyKey, referenceValue string, body any, expiration time.Duration) (bool, error) {
	if b.stallBody {
		b.stalls++
		<-ctx.Done()
		return false, ctx.Err()
	}
	return b.CacheClient.(interface {
		StoreVersionedBody(context.Context, string, string, string, any, time.Duration) (bool, error)
	}).StoreVersionedBody(ctx, referenceKey, bodyKey, referenceValue, body, expiration)
}

func newIndexProtocolCache(t *testing.T) (*Cache, *indexProtocolBackend) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	backend := &indexProtocolBackend{CacheClient: cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone)}
	return NewCache(cacheutil.NewCache(backend), time.Hour, time.Minute, time.Minute), backend
}

func indexProtocolOrigin(t *testing.T) (string, *atomic.Int32, *atomic.Int32, []byte) {
	t.Helper()
	archive, err := os.ReadFile(filepath.Join("..", "..", "test", "e2e", "testdata", "helm-repo", "helm-1.0.0.tgz"))
	require.NoError(t, err)
	body := []byte("apiVersion: v1\nentries:\n  helm:\n    - name: helm\n      apiVersion: v1\n      version: 1.0.0\n      urls: [helm-1.0.0.tgz]\n")
	var requests, archives atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/helm-1.0.0.tgz" {
			archives.Add(1)
			_, _ = w.Write(archive)
			return
		}
		requests.Add(1)
		w.Header().Set("ETag", `"one"`)
		_, _ = w.Write(body)
	}))
	t.Cleanup(origin.Close)
	return origin.URL, &requests, &archives, body
}

func indexProtocolBuild(t *testing.T, ctx context.Context, url string, store *helm.IndexStore, force bool) {
	t.Helper()
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: helm\n    version: 1.0.0\n    repository: "+url+"\n"), 0o600))
	app, err := helm.NewHelmAppWithIndexStore(parent, []helm.HelmRepository{{Name: "charts", Repo: url, Creds: helm.HelmCreds{}}}, false, "", "", "", false, false, store, force)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.NoError(t, app.DependencyBuild(ctx))
	require.NoError(t, ctx.Err(), "cache work must leave time for the Helm subprocess")
}

func TestHelmIndexPublicationBudgets(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, force := range []bool{false, true} {
		for _, stage := range []string{"metadata", "body"} {
			t.Run(stage+"/force="+strconv.FormatBool(force), func(t *testing.T) {
				shared, backend := newIndexProtocolCache(t)
				url, requests, archives, body := indexProtocolOrigin(t)
				identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
				if stage == "metadata" {
					require.NoError(t, shared.SetHelmIndexRetainedRecord(identity, &HelmIndexRecord{Body: body, StoredAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}))
					backend.stallMetadata = true
				} else {
					backend.stallBody = true
				}
				store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				indexProtocolBuild(t, ctx, url, store, force)
				require.EqualValues(t, 1, requests.Load())
				require.EqualValues(t, 1, archives.Load())
				require.Equal(t, 1, backend.stalls)
				_, err := store.Get(identity)
				if stage == "metadata" {
					require.Error(t, err, "unconfirmed publication cannot advance disk freshness")
					return
				}
				require.NoError(t, err, "confirmed metadata permits local warming when body sharing times out")
				_, _, err = shared.GetHelmIndexSnapshotContext(t.Context(), identity)
				require.ErrorIs(t, err, ErrCacheMiss, "cold readers cannot treat a missing shared body as a hit")
				reads := backend.bodyReads
				indexProtocolBuild(t, t.Context(), url, store, false)
				require.EqualValues(t, 1, requests.Load(), "the next build must use its confirmed local body")
				require.Equal(t, reads, backend.bodyReads, "matching metadata must avoid full Redis body reads")
			})
		}
	}
}

func TestHelmIndexSlowBodyDoesNotCauseRepeatedDownloads(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	shared, backend := newIndexProtocolCache(t)
	url, requests, _, body := indexProtocolOrigin(t)
	identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
	require.NoError(t, shared.SetHelmIndexRecord(identity, body, `"one"`, ""))
	backend.bodyDelay = 1200 * time.Millisecond
	store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	indexProtocolBuild(t, ctx, url, store, false)
	indexProtocolBuild(t, ctx, url, store, false)
	require.EqualValues(t, 0, requests.Load(), "a shared body taking over one second must remain usable")
	require.Equal(t, 1, backend.bodyReads, "the second build must only read metadata")
}

func TestHelmIndexLegacyMissesShareOneBudget(t *testing.T) {
	shared, backend := newIndexProtocolCache(t)
	backend.missDelay = 450 * time.Millisecond
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(helmIndexContractBody("1.0.0"))
	}))
	t.Cleanup(origin.Close)
	client := helm.NewClient(origin.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(shared))
	ctx, cancel := context.WithTimeout(t.Context(), 800*time.Millisecond)
	defer cancel()
	_, err := client.GetIndex(ctx, false, 1<<20)
	require.NoError(t, err)
	require.NoError(t, ctx.Err())
	require.EqualValues(t, 1, requests.Load())
}

func TestHelmIndexChangedGenerationRequiresUsableBody(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, corrupt := range []bool{false, true} {
		t.Run("corrupt="+strconv.FormatBool(corrupt), func(t *testing.T) {
			shared, _ := newIndexProtocolCache(t)
			url, requests, _, body := indexProtocolOrigin(t)
			identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
			store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
			indexProtocolBuild(t, t.Context(), url, store, false)
			require.EqualValues(t, 1, requests.Load())
			// A newer shared generation invalidates the still-fresh local one.
			// An old legacy body must not hide a missing/corrupt current body.
			require.NoError(t, shared.cache.SetItem(helmIndexRefsKey(identity), body, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
			require.NoError(t, shared.SetHelmIndexRecord(identity, append(append([]byte{}, body...), []byte("# newer\n")...), `"newer"`, ""))
			_, err := shared.GetHelmIndexRecord(identity)
			require.NoError(t, err)
			key := helmIndexBodyKey(identity)
			require.NoError(t, shared.cache.SetItem(key, []byte("wrong body"), &cacheutil.CacheActionOpts{Expiration: time.Hour, Delete: !corrupt}))
			_, _, err = shared.GetHelmIndexSnapshotContext(t.Context(), identity)
			require.Error(t, err, "legacy data must not hide an unusable current generation")
			indexProtocolBuild(t, t.Context(), url, store, false)
			require.EqualValues(t, 2, requests.Load(), "a changed generation without a usable body requires origin validation")
		})
	}
}

func TestHelmIndexMetadataCannotSelectAnOlderBody(t *testing.T) {
	shared, _ := newIndexProtocolCache(t)
	repo := "https://charts.example.test"
	require.NoError(t, shared.SetHelmIndex(repo, helmIndexContractBody("1.0.0")))
	owner, err := shared.BeginHelmIndexFetch(t.Context(), repo)
	require.NoError(t, err)
	updated, err := shared.PublishHelmIndexRecord(t.Context(), repo, owner, &HelmIndexRecord{Body: helmIndexContractBody("2.0.0"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)})
	require.NoError(t, err)
	require.True(t, updated)
	record, err := shared.GetHelmIndexRecord(repo)
	require.NoError(t, err)
	bodyKey := helmIndexBodyKey(repo)
	for _, corrupt := range []bool{false, true} {
		if corrupt {
			require.NoError(t, shared.cache.SetItem(bodyKey, helmIndexContractBody("1.0.0"), nil))
		} else {
			require.NoError(t, shared.cache.SetItem(bodyKey, record.Body, &cacheutil.CacheActionOpts{Delete: true}))
		}
		_, fresh, err := shared.GetHelmIndexSnapshotContext(t.Context(), repo)
		require.Error(t, err)
		require.False(t, fresh, "legacy raw data cannot replace the current generation's missing or corrupt body")
	}
}

func TestHelmIndexZeroExpirationDoesNotReuseLegacyFreshness(t *testing.T) {
	shared, _ := newIndexProtocolCache(t)
	shared.helmIndexCacheExpiration = 0
	repo := "https://charts.example.test"
	body := helmIndexContractBody("1.0.0")
	require.NoError(t, shared.cache.SetItem(helmIndexRefsKey(repo), body, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
	record, fresh, err := shared.GetHelmIndexSnapshotContext(t.Context(), repo)
	require.NoError(t, err)
	require.Equal(t, body, record.Body)
	require.False(t, fresh, "zero expiration must require origin validation even for a legacy hit")
	var cached []byte
	require.ErrorIs(t, shared.GetHelmIndex(repo, &cached), ErrCacheMiss)
}

func TestHelmIndexSharedBodyStorageIsBounded(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	shared := NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone)), time.Hour, time.Minute, time.Minute)
	repo := "https://charts.example.test"
	for version := range 10 {
		require.NoError(t, shared.SetHelmIndexRecord(repo, helmIndexContractBody(strconv.Itoa(version)+".0.0"), "", ""))
		// Evict only the small keys. Losing them must not orphan a large body
		// for every subsequent generation during the retention window.
		for _, key := range server.Keys() {
			if strings.Contains(key, "|record") || strings.Contains(key, "|body-reference") {
				server.Del(key)
			}
		}
		require.Len(t, server.Keys(), 1, "only the current large body may remain after metadata eviction")
	}
}

func TestHelmIndexLocalBodiesStayWarmAcrossReplicasWhenSharingStalls(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	shared, backend := newIndexProtocolCache(t)
	backend.stallBody = true
	url, requests, _, _ := indexProtocolOrigin(t)
	stores := []*helm.IndexStore{
		helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared),
		helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared),
	}
	for build := range 4 {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		indexProtocolBuild(t, ctx, url, stores[build%2], false)
		cancel()
		require.EqualValues(t, min(build+1, 2), requests.Load(), "a new shared validation of identical bytes must not make an already-warm replica cold")
	}
	require.Equal(t, 2, backend.stalls, "only the first cold build on each replica attempts body sharing")
	require.Equal(t, 1, backend.bodyReads, "matching local bytes must avoid another full shared-body read")
}
