package helm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

func TestIndexValidationWarmBodyAvoidsYAMLAllocations(t *testing.T) {
	body := []byte(regressionIndexBody + "# " + t.Name() + "\n")
	require.NoError(t, validateIndexBody(body))
	var err error
	allocations := testing.AllocsPerRun(5, func() { err = validateIndexBody(body) })
	require.NoError(t, err)
	require.Less(t, allocations, 10.0, "unchanged bodies must not allocate another YAML tree on each cache read")
}

func TestIndexValidationMemoizationIsBoundedAndRetriesFailures(t *testing.T) {
	var calls int
	cache := newIndexValidationCache(2, func(body []byte) error {
		calls++
		return parseIndexBody(body)
	})
	a := []byte(regressionIndexBody + "# a\n")
	b := []byte(regressionIndexBody + "# b\n")
	c := []byte(regressionIndexBody + "# c\n")
	for _, body := range [][]byte{a, b, a, c, a} {
		require.NoError(t, cache.validateBody(body))
	}
	require.Equal(t, 3, calls, "warm and recently used bodies must not be reparsed")
	require.NoError(t, cache.validateBody(b))
	require.Equal(t, 4, calls, "evicted bodies must be parsed again")
	require.Equal(t, 2, cache.valid.Len())
	for range 2 {
		require.Error(t, cache.validateBody([]byte("apiVersion: v9\nentries: {}\n")))
	}
	require.Equal(t, 6, calls, "failed validation must never become a successful cache hit")
	require.Equal(t, 2, cache.valid.Len())
}

func TestIndexValidationConcurrentMissesLeaveWarmBodiesAvailable(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	t.Cleanup(resume)
	cache := newIndexValidationCache(2, func(body []byte) error {
		if calls.Add(1) == 2 {
			close(started)
			<-release
		}
		return parseIndexBody(body)
	})
	warm := []byte(regressionIndexBody + "# warm\n")
	cold := []byte(regressionIndexBody + "# cold\n")
	require.NoError(t, cache.validateBody(warm))
	const workers = 16
	results := make(chan error, workers)
	for range workers {
		go func() { results <- cache.validateBody(cold) }()
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("cold validation did not start")
	}
	warmResult := make(chan error, 1)
	go func() { warmResult <- cache.validateBody(warm) }()
	select {
	case err := <-warmResult:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("cold validation blocked an unrelated warm body")
	}
	resume()
	for range workers {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent validations did not finish")
		}
	}
	require.EqualValues(t, 2, calls.Load(), "identical concurrent cold bodies must share one validation")
}

func TestIndexValidationDetectsCorruptionWithUnchangedMetadata(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	valid := []byte(regressionIndexBody + "# " + t.Name() + "\n")
	require.NoError(t, store.Set("view", valid))
	_, err := store.Get("view")
	require.NoError(t, err)
	file := store.cacheFileName("view")
	info, err := os.Stat(file)
	require.NoError(t, err)
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	var record indexRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	corrupt := bytes.Replace(valid, []byte("apiVersion: v1"), []byte("apiVersion: v9"), 1)
	require.Len(t, corrupt, len(valid))
	record.Body = base64.StdEncoding.EncodeToString(corrupt)
	raw, err = json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, raw, 0o600))
	require.NoError(t, os.Chtimes(file, info.ModTime(), info.ModTime()))
	_, err = store.Get("view")
	require.ErrorContains(t, err, "digest", "same-size changes must be rejected even when timestamps are unchanged")
	_, err = store.GetRetained("view")
	require.ErrorContains(t, err, "digest")
}

func TestIndexValidationRejectsNewerCorruptSharedRecord(t *testing.T) {
	shared := &providerRecordCache{}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	body := []byte(regressionIndexBody + "# " + t.Name() + "\n")
	require.NoError(t, store.Set("view", body))
	local, err := store.readRecord("view", false)
	require.NoError(t, err)
	validatedAt, err := time.Parse(time.RFC3339Nano, local.StoredAt)
	require.NoError(t, err)
	for _, candidate := range [][]byte{[]byte("apiVersion: v9\nentries: {}\n"), []byte("apiVersion: v1\nentries: [")} {
		require.NoError(t, shared.SetHelmIndexRetainedRecord("view", &HelmIndexRecord{
			Body: candidate, StoredAt: validatedAt.Add(time.Millisecond).Format(time.RFC3339Nano),
		}))
		got, fromShared := store.retainedRecord("view")
		require.NotNil(t, got)
		require.False(t, fromShared)
		require.Equal(t, body, got.Body)
		require.Equal(t, local.StoredAt, got.StoredAt, "rejected shared data must not advance validation time")
	}
}

func benchmarkIndexBody(b *testing.B) []byte {
	b.Helper()
	var body strings.Builder
	body.Grow(27 << 20)
	body.WriteString("apiVersion: v1\nentries:\n  fixture:\n")
	description := strings.Repeat("representative chart description ", 80)
	for i := 0; body.Len() < 27<<20; i++ {
		fmt.Fprintf(&body, "    - apiVersion: v2\n      name: fixture\n      version: 1.0.%d\n      urls: [fixture-1.0.%d.tgz]\n      description: %s\n", i, i, description)
	}
	return []byte(body.String())
}

func BenchmarkIndexValidation27MiB(b *testing.B) {
	body := benchmarkIndexBody(b)
	require.NoError(b, validateIndexBody(body))
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for range b.N {
		require.NoError(b, validateIndexBody(body))
	}
}

func BenchmarkIndexStoreWarm27MiB(b *testing.B) {
	body := benchmarkIndexBody(b)
	shared := &providerRecordCache{record: &HelmIndexRecord{Body: body, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	store := NewIndexStore(b.TempDir(), time.Hour, 32<<20, shared)
	require.NoError(b, store.Set("view", body))
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for range b.N {
		record, _ := store.retainedRecord("view")
		if record == nil || !bytes.Equal(body, record.Body) {
			b.Fatal("warm store did not preserve the original index body")
		}
	}
}

type providerRecordCache struct {
	mu     sync.Mutex
	record *HelmIndexRecord
	err    error
}

func (c *providerRecordCache) GetHelmIndexRetainedRecord(string) (*HelmIndexRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	if c.record == nil {
		return nil, cacheutil.ErrCacheMiss
	}
	cloned := *c.record
	cloned.Body = bytes.Clone(cloned.Body)
	return &cloned, nil
}

func (c *providerRecordCache) SetHelmIndexRetainedRecord(_ string, record *HelmIndexRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	cloned := *record
	cloned.Body = bytes.Clone(cloned.Body)
	c.record = &cloned
	return nil
}

func TestIndexProviderUsesSharedPublicationOrder(t *testing.T) {
	for _, sharedNewer := range []bool{false, true} {
		t.Run(map[bool]string{false: "disk newer", true: "Redis newer"}[sharedNewer], func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(server.Close)
			diskTime := time.Now().Add(-20 * time.Minute).UTC()
			sharedTime := diskTime.Add(-10 * time.Minute)
			if sharedNewer {
				sharedTime = diskTime.Add(10 * time.Minute)
			}
			want := regressionIndexBody + "# Redis\n"
			wantTime := sharedTime
			shared := &providerRecordCache{record: &HelmIndexRecord{Body: []byte(regressionIndexBody + "# Redis\n"), StoredAt: sharedTime.Format(time.RFC3339Nano)}}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
			require.NoError(t, store.publishRecord("view", &indexRecord{APIVersion: "v1", Body: base64.StdEncoding.EncodeToString([]byte(regressionIndexBody + "# disk\n")), StoredAt: diskTime.Format(time.RFC3339Nano)}))
			client := NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart)
			body, err := store.Revalidate(t.Context(), "view", client)
			require.NoError(t, err)
			require.Equal(t, want, string(body))
			require.Zero(t, requests.Load())
			record, err := store.readRecord("view", true)
			require.NoError(t, err)
			require.Equal(t, wantTime.Format(time.RFC3339Nano), record.StoredAt, "copying must preserve the shared validation time")
		})
	}
}

func TestIndexProviderCacheOutageAnd304ValidatorUpdate(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") != `"one"` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("ETag", `W/"one"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	shared := &providerRecordCache{err: errors.New("Redis unavailable")}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	require.NoError(t, store.publishRecord("view", &indexRecord{APIVersion: "v1", ETag: `"one"`, URLIdentity: IndexURLIdentity(server.URL + "/index.yaml"), Body: base64.StdEncoding.EncodeToString([]byte(regressionIndexBody)), StoredAt: old}))
	body, err := store.Revalidate(t.Context(), "view", NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart))
	require.NoError(t, err)
	require.Equal(t, regressionIndexBody, string(body))
	require.EqualValues(t, 1, requests.Load())
	etag, _, err := store.GetHelmIndexValidators("view")
	require.NoError(t, err)
	require.Equal(t, `"one"`, etag, "an outage cannot confirm a new shared deadline or metadata")
	_, err = store.Get("view")
	require.Error(t, err, "disk must remain expired while shared publication is unconfirmed")
}

func TestIndexProviderPublicationFailurePreservesHTTPResult(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions and a non-root user")
	}
	dir := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The lock already exists. Fail only publication after HTTP succeeds.
		if err := os.Chmod(dir, 0o500); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	store := NewIndexStore(dir, time.Hour, 1<<20)
	body, err := store.Revalidate(t.Context(), "view", NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart))
	require.NoError(t, err)
	require.Equal(t, regressionIndexBody, string(body))
	_, err = os.Stat(store.cacheFileName("view"))
	require.ErrorIs(t, err, os.ErrNotExist, "the test must actually fail disk publication")
}

func TestIndexCacheIdentityPreservesRepositoryViews(t *testing.T) {
	scope := credsScope(HelmCreds{Password: "secret"})
	key := func(url, proxy, noProxy string) string { return scopedIndexIdentity(url, scope, proxy, noProxy) }
	require.Equal(t, key("https://EXAMPLE.com/repo/", "", ""), key("https://example.com/repo", "", ""))
	for _, other := range []string{"https://example.com/other", "https://example.com/repo?tenant=one", "https://example.com/repo?tenant=two", "https://example.com/re%2Fpo"} {
		require.NotEqual(t, key("https://example.com/repo", "", ""), key(other, "", ""))
	}
	require.NotEqual(t, key("https://example.com/repo?tenant=one", "", ""), key("https://example.com/repo?tenant=two", "", ""))
	require.NotEqual(t, key("https://example.com/repo", "", ""), key("https://example.com/repo", "http://proxy", ""))
	require.NotEqual(t, key("https://example.com/repo", "", ""), key("https://example.com/repo", "", "example.com"))
	private := key("https://reader:private@example.com/repo?token=sensitive", "http://user:password@proxy", "")
	for _, secret := range []string{"private", "sensitive", "password", "secret"} {
		require.NotContains(t, private, secret)
	}
}

func TestIndexCacheIdentityIncludesInheritedProxyAndUnambiguousCredentials(t *testing.T) {
	creds := HelmCreds{Username: "reader", Password: "password"}
	t.Setenv("HTTPS_PROXY", "http://user:first@proxy.example")
	first := credsScope(creds)
	t.Setenv("HTTPS_PROXY", "http://user:second@proxy.example")
	second := credsScope(creds)
	require.NotEqual(t, first, second, "pods with different inherited proxy authentication must not share indexes")
	require.NotContains(t, first, "first")
	require.NotContains(t, second, "second")
	require.NotEqual(t, credsScope(HelmCreds{Username: "a\x00b", Password: "c"}), credsScope(HelmCreds{Username: "a", Password: "b\x00c"}), "field separators must not create credential collisions")
}

func TestIndexClientLockWaitRespectsCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	t.Cleanup(resume)
	client := NewClient(server.URL, HelmCreds{}, false, "", "")
	completed := make(chan error, 1)
	go func() { _, err := client.GetIndex(t.Context(), true, 1<<20); completed <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first fetch did not reach origin")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	waiter := make(chan error, 1)
	go func() { _, err := client.GetIndex(ctx, true, 1<<20); waiter <- err }()
	select {
	case err := <-waiter:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(3 * time.Second):
		resume()
		t.Fatal("the lock waiter ignored cancellation")
	}
	require.EqualValues(t, 1, requests.Load())
	resume()
	select {
	case err := <-completed:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("the original request did not finish after release")
	}
}

func TestIndexStoreRejectsUnsafeSnapshotName(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	err := store.CopyTo("view", []byte(regressionIndexBody), t.TempDir(), "../outside")
	require.Error(t, err)
}

func TestIndexProviderPreservesLargeOriginalBody(t *testing.T) {
	body := append([]byte("apiVersion: v1\nentries: {}\nx-extension: preserve-me\n# "), bytes.Repeat([]byte("x"), 6<<20)...)
	body = append(body, '\n')
	var requests atomic.Int32
	var transferred atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"large"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"large"`)
		transferred.Add(int64(len(body)))
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 7<<20)
	client := NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart)
	for range 2 {
		got, err := store.Revalidate(t.Context(), "large", client)
		require.NoError(t, err)
		require.Equal(t, body, got, "unknown fields and original formatting must survive")
	}
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(store.cacheFileName("large"), old, old))
	got, err := store.Revalidate(t.Context(), "large", client)
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.EqualValues(t, 2, requests.Load())
	require.EqualValues(t, len(body), transferred.Load(), "only the cold GET transfers the 6 MB body")
}

func TestIndexRepositoryConfigRejectsConflictingCredentials(t *testing.T) {
	command := newTestCmd(t)
	err := command.writeRepoEntries([]helmRepoEntry{
		{Name: "same", URL: "https://example.com", Password: "first"},
		{Name: "same", URL: "https://example.com", Password: "second"},
	})
	require.ErrorContains(t, err, "conflicting credentials")
}

type providerReadRaceCache struct {
	*providerRecordCache
	newer  *HelmIndexRecord
	writes int
}

func (c *providerReadRaceCache) GetHelmIndexRetainedRecord(key string) (*HelmIndexRecord, error) {
	observed, err := c.providerRecordCache.GetHelmIndexRetainedRecord(key)
	if err == nil {
		err = c.providerRecordCache.SetHelmIndexRetainedRecord(key, c.newer)
	}
	return observed, err
}

func (c *providerReadRaceCache) SetHelmIndexRetainedRecord(key string, record *HelmIndexRecord) error {
	c.writes++
	return c.providerRecordCache.SetHelmIndexRetainedRecord(key, record)
}

func TestIndexProviderSharedHitCannotRollBackNewerRecord(t *testing.T) {
	old := &HelmIndexRecord{Body: []byte(regressionIndexBody + "# old\n"), StoredAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}
	newer := &HelmIndexRecord{Body: []byte(regressionIndexBody + "# new\n"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	shared := &providerReadRaceCache{providerRecordCache: &providerRecordCache{record: old}, newer: newer}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	body, err := store.Revalidate(t.Context(), "view", regressionClient("https://unused.invalid"))
	require.NoError(t, err)
	require.Equal(t, old.Body, body)
	require.Zero(t, shared.writes, "a Redis hit is copied to disk without being published back")
	record, err := shared.providerRecordCache.GetHelmIndexRetainedRecord("view")
	require.NoError(t, err)
	require.Equal(t, newer.Body, record.Body)
	local, err := store.readRecord("view", true)
	require.NoError(t, err)
	require.Equal(t, old.StoredAt, local.StoredAt)
}

func TestIndexProvider304WithoutSharedComparisonPreservesNewerRecord(t *testing.T) {
	old := &HelmIndexRecord{Body: []byte(regressionIndexBody + "# old\n"), ETag: `"one"`, StoredAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
	newer := &HelmIndexRecord{Body: []byte(regressionIndexBody + "# new\n"), ETag: `"two"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	shared := &providerRecordCache{record: old}
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"one"` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		result <- shared.SetHelmIndexRetainedRecord("view", newer)
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	old.URLIdentity = IndexURLIdentity(server.URL + "/index.yaml")
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	body, err := store.Revalidate(t.Context(), "view", regressionClient(server.URL))
	require.NoError(t, err)
	require.Equal(t, old.Body, body)
	select {
	case err := <-result:
		require.NoError(t, err)
	default:
		t.Fatal("no conditional request")
	}
	record, err := shared.GetHelmIndexRetainedRecord("view")
	require.NoError(t, err)
	require.Equal(t, newer.Body, record.Body)
	_, err = store.readRecord("view", true)
	require.Error(t, err, "unsupported shared publication cannot renew disk freshness")
	next, err := store.Revalidate(t.Context(), "view", regressionClient(server.URL))
	require.NoError(t, err)
	require.Equal(t, newer.Body, next, "the next build must select the shared winner")
}

type atomicProviderRecordCache struct {
	*providerRecordCache
	compareErr   error
	claimErr     error
	publishCalls int
	owner        string
	nextOwner    int
}

type metadataProviderRecordCache struct {
	*atomicProviderRecordCache
	missingBody bool
}

type indexChangedBeforeClaimCache struct {
	*atomicProviderRecordCache
	next  *HelmIndexRecord
	fresh bool
}

func (c *indexChangedBeforeClaimCache) GetHelmIndex(key string, body *[]byte) error {
	record, err := c.GetHelmIndexRetainedRecord(key)
	if err == nil {
		*body = record.Body
	}
	return err
}

func (c *indexChangedBeforeClaimCache) SetHelmIndex(string, []byte) error {
	return errors.New("unfenced publication must not be used")
}

func (c *indexChangedBeforeClaimCache) GetHelmIndexSnapshotContext(_ context.Context, key string) (*HelmIndexRecord, bool, error) {
	record, err := c.GetHelmIndexRetainedRecord(key)
	return record, c.fresh, err
}

func (c *indexChangedBeforeClaimCache) BeginHelmIndexFetch(ctx context.Context, key string) (string, error) {
	if c.next != nil {
		if err := c.SetHelmIndexRetainedRecord(key, c.next); err != nil {
			return "", err
		}
	}
	return c.atomicProviderRecordCache.BeginHelmIndexFetch(ctx, key)
}

type recordedIndexHeaders struct {
	mu     sync.Mutex
	values []string
}

func (h *recordedIndexHeaders) record(value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
}

func (h *recordedIndexHeaders) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.values...)
}

func TestIndexRevalidationObservesRecordAfterClaim(t *testing.T) {
	for _, disk := range []bool{false, true} {
		t.Run(strconv.FormatBool(disk), func(t *testing.T) {
			headers := &recordedIndexHeaders{}
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				header := r.Header.Get("If-None-Match")
				headers.record(header)
				if header == `"one"` {
					w.Header().Set("ETag", `"one"`)
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set("ETag", `"two"`)
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(origin.Close)
			stamp := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
			identity := IndexURLIdentity(origin.URL + "/index.yaml")
			first := &HelmIndexRecord{Body: upstreamIndex(1), ETag: `"one"`, Generation: "one", StoredAt: stamp, URLIdentity: identity}
			next := &HelmIndexRecord{Body: upstreamIndex(2), ETag: `"two"`, Generation: "two", StoredAt: stamp, URLIdentity: identity}
			shared := &indexChangedBeforeClaimCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: first}}, next: next}
			if disk {
				store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
				body, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
				require.NoError(t, err)
				require.Equal(t, next.Body, body)
			} else {
				client := NewClient(origin.URL, HelmCreds{}, false, "", "", WithIndexCache(shared))
				index, err := client.GetIndex(t.Context(), false, 1<<20)
				require.NoError(t, err)
				require.Equal(t, "2.0.0", index.Entries["child"][0].Version)
			}
			require.Equal(t, []string{`"two"`}, headers.snapshot(), "a pre-claim observation cannot authorize a 304")
			record, err := shared.GetHelmIndexRetainedRecord("view")
			require.NoError(t, err)
			require.Equal(t, next.Body, record.Body)
		})
	}
}

func (c *metadataProviderRecordCache) GetHelmIndexMetadataContext(context.Context, string) (*HelmIndexRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	metadata := *c.record
	digest := sha256.Sum256(metadata.Body)
	metadata.BodyDigest = hex.EncodeToString(digest[:])
	metadata.Body = nil
	return &metadata, nil
}

func (c *metadataProviderRecordCache) GetHelmIndexRetainedRecord(repo string) (*HelmIndexRecord, error) {
	if c.missingBody {
		return nil, cacheutil.ErrCacheMiss
	}
	return c.providerRecordCache.GetHelmIndexRetainedRecord(repo)
}

func TestIndexProviderRepairsAlteredValidatorMetadata(t *testing.T) {
	body := upstreamIndex(1)
	headers := &recordedIndexHeaders{}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("If-None-Match")
		headers.record(header)
		w.Header().Set("ETag", `"two"`)
		if header == `"two"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(upstreamIndex(2))
	}))
	t.Cleanup(origin.Close)
	record := &HelmIndexRecord{Body: body, Generation: "original", ETag: `"one"`, URLIdentity: IndexURLIdentity(origin.URL + "/index.yaml"), StoredAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
	shared := &metadataProviderRecordCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: record}}, missingBody: true}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	local := *record
	local.ETag = `"two"`
	require.True(t, store.installRecord("view", &local))
	_, err := store.readRecord("view", false)
	require.NoError(t, err, "the conflicting local metadata must pass its own checksum")
	got, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
	require.NoError(t, err)
	require.Equal(t, upstreamIndex(2), got, "generation equality cannot validate altered local headers")
	require.Equal(t, []string{`"one"`}, headers.snapshot(), "conditional headers must come from the confirmed shared metadata")
}

func TestIndexProviderRejectsValidYAMLWithWrongDigest(t *testing.T) {
	for _, expired := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			t.Run(fmt.Sprintf("expired=%v/missing=%v", expired, missing), func(t *testing.T) {
				body := upstreamIndex(1)
				var requests, conditional atomic.Int32
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					w.Header().Set("ETag", `"one"`)
					if r.Header.Get("If-None-Match") != "" {
						conditional.Add(1)
						w.WriteHeader(http.StatusNotModified)
						return
					}
					_, _ = w.Write(body)
				}))
				t.Cleanup(origin.Close)
				stamp := time.Now()
				if expired {
					stamp = stamp.Add(-2 * time.Hour)
				}
				record := &HelmIndexRecord{Body: body, Generation: "original", ETag: `"one"`, URLIdentity: IndexURLIdentity(origin.URL + "/index.yaml"), StoredAt: stamp.UTC().Format(time.RFC3339Nano)}
				shared := &metadataProviderRecordCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: record}}, missingBody: missing}
				store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
				local := *record
				local.Body = upstreamIndex(2)
				require.True(t, store.installRecord("view", &local))
				_, err := store.readRecord("view", false)
				require.NoError(t, err, "exercise the shared digest comparison, not local corruption rejection")
				got, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
				require.NoError(t, err)
				require.Equal(t, body, got, "valid YAML must not bypass the authoritative body digest")
				if missing {
					require.EqualValues(t, 1, requests.Load())
					require.Zero(t, conditional.Load(), "an altered body cannot authorize a 304")
				}
			})
		}
	}
}

func (c *atomicProviderRecordCache) BeginHelmIndexFetch(_ context.Context, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.claimErr != nil {
		return "", c.claimErr
	}
	if c.owner != "" {
		return "", nil
	}
	c.nextOwner++
	c.owner = strconv.Itoa(c.nextOwner)
	return c.owner, nil
}

func (c *atomicProviderRecordCache) ReleaseHelmIndexFetch(_ context.Context, _ string, owner string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.owner == owner {
		c.owner = ""
	}
	return nil
}

func (c *atomicProviderRecordCache) PublishHelmIndexRecord(_ context.Context, _ string, owner string, replacement *HelmIndexRecord) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.publishCalls++
	if c.compareErr != nil {
		return false, c.compareErr
	}
	if owner == "" || owner != c.owner {
		return false, nil
	}
	c.owner = ""
	cloned := *replacement
	cloned.Body = bytes.Clone(replacement.Body)
	c.record = &cloned
	return true, nil
}

func TestIndexProviderPublicationConfirmation(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotModified} {
		for _, mode := range []string{"matching", "mismatch", "error", "unsupported", "evicted", "evicted-after-update", "absent", "read-error", "claim-error"} {
			t.Run(fmt.Sprintf("%d/%s", status, mode), func(t *testing.T) {
				old := &HelmIndexRecord{Body: upstreamIndex(0), ETag: `"one"`, StoredAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
				newer := &HelmIndexRecord{Body: upstreamIndex(2), ETag: `"two"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
				responseBody, responseETag := old.Body, old.ETag
				if status == http.StatusOK {
					responseBody, responseETag = upstreamIndex(1), `"response-one"`
				}
				base := &providerRecordCache{record: old}
				atomicCache := &atomicProviderRecordCache{providerRecordCache: base}
				var shared IndexRecordCache = atomicCache
				if mode == "unsupported" {
					shared = base
				}
				if mode == "error" {
					atomicCache.compareErr = errors.New("comparison unavailable")
				}
				if mode == "claim-error" {
					atomicCache.claimErr = errors.New("claim unavailable")
				}
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					request := requests.Add(1)
					if r.Header.Get("If-None-Match") != `"one"` {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if request == 2 && mode == "evicted-after-update" {
						w.Header().Set("ETag", newer.ETag)
						_, _ = w.Write(newer.Body)
						return
					}
					base.mu.Lock()
					switch mode {
					case "evicted":
						base.record = nil
					case "evicted-after-update":
						atomicCache.owner = ""
						base.record = nil
					case "mismatch", "error", "unsupported", "claim-error":
						atomicCache.owner = ""
						base.record = newer
					}
					base.err = nil
					base.mu.Unlock()
					w.Header().Set("ETag", responseETag)
					w.WriteHeader(status)
					if status == http.StatusOK {
						_, _ = w.Write(responseBody)
					}
				}))
				t.Cleanup(server.Close)
				old.URLIdentity = IndexURLIdentity(server.URL + "/index.yaml")
				store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
				require.True(t, store.installRecord("view", old))
				if mode == "absent" {
					base.record = nil
				}
				if mode == "read-error" {
					base.err = errors.New("shared read failed")
				}
				body, err := store.Revalidate(t.Context(), "view", regressionClient(server.URL))
				require.NoError(t, err)
				require.Equal(t, responseBody, body, "a 200 must return its downloaded body")
				if mode == "claim-error" {
					base.mu.Lock()
					calls := atomicCache.publishCalls
					base.mu.Unlock()
					require.Zero(t, calls, "a failed claim cannot attempt publication")
				}
				if mode == "read-error" {
					base.mu.Lock()
					calls := atomicCache.publishCalls
					base.mu.Unlock()
					require.Equal(t, 1, calls, "a failed read does not prevent publication after a confirmed claim")
				}
				local, localErr := store.Get("view")
				retained, sharedErr := shared.GetHelmIndexRetainedRecord("view")
				wantNext := newer.Body
				wantRequests := int32(1)
				switch mode {
				case "matching", "absent", "evicted", "read-error":
					require.NoError(t, localErr)
					require.Equal(t, responseBody, local)
					require.NoError(t, sharedErr)
					require.Equal(t, responseBody, retained.Body)
					require.Equal(t, responseETag, retained.ETag)
					require.True(t, retained.Fresh(time.Hour))
					wantNext = responseBody
				case "evicted-after-update":
					require.Error(t, localErr)
					require.ErrorIs(t, sharedErr, cacheutil.ErrCacheMiss, "a vanished observed generation must not be revived")
					wantRequests = 2
				default:
					require.Error(t, localErr, "unconfirmed publication must leave disk expired")
					require.NoError(t, sharedErr)
					require.Equal(t, newer.Body, retained.Body)
				}
				next, err := store.Revalidate(t.Context(), "view", regressionClient(server.URL))
				require.NoError(t, err)
				require.Equal(t, wantNext, next)
				require.Equal(t, wantRequests, requests.Load())
				if mode == "evicted-after-update" {
					retained, err = shared.GetHelmIndexRetainedRecord("view")
					require.NoError(t, err)
					require.Equal(t, newer.Body, retained.Body)
					require.True(t, retained.Fresh(time.Hour))
				}
			})
		}
	}
}

func TestLatestSharedPublicationWinsWithClockSkew(t *testing.T) {
	first := &HelmIndexRecord{Body: upstreamIndex(1), StoredAt: time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)}
	base := &providerRecordCache{record: first}
	shared := &atomicProviderRecordCache{providerRecordCache: base}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	body, err := store.Revalidate(t.Context(), "view", regressionClient("https://unused.invalid"))
	require.NoError(t, err)
	require.Equal(t, first.Body, body)
	// A second pod with a clock 100ms behind publishes a later generation.
	timestamp, err := time.Parse(time.RFC3339Nano, first.StoredAt)
	require.NoError(t, err)
	next := &HelmIndexRecord{Body: upstreamIndex(2), StoredAt: timestamp.Add(-100 * time.Millisecond).Format(time.RFC3339Nano)}
	owner, err := shared.BeginHelmIndexFetch(t.Context(), "view")
	require.NoError(t, err)
	updated, err := shared.PublishHelmIndexRecord(t.Context(), "view", owner, next)
	require.NoError(t, err)
	require.True(t, updated)
	body, err = store.Revalidate(t.Context(), "view", regressionClient("https://unused.invalid"))
	require.NoError(t, err)
	require.Equal(t, next.Body, body, "a later shared publication must not lose to a stale local timestamp")
}

func TestIndexProviderIdenticalSharedHitDoesNotRewriteDisk(t *testing.T) {
	record := &HelmIndexRecord{Body: upstreamIndex(1), ETag: `"one"`, URLIdentity: IndexURLIdentity("https://example.com/index.yaml"), StoredAt: time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)}
	shared := &providerRecordCache{record: record}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	client := NewClient("https://example.com", HelmCreds{}, false, "", "").(*nativeHelmChart)
	_, err := store.Revalidate(t.Context(), "view", client)
	require.NoError(t, err)
	file := store.cacheFileName("view")
	// readRecord uses the earlier of mtime and StoredAt, so moving mtime
	// forward does not change this record's freshness or shared identity.
	marker := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(file, marker, marker))
	before, err := os.Stat(file)
	require.NoError(t, err)
	for range 2 {
		body, err := store.Revalidate(t.Context(), "view", client)
		require.NoError(t, err)
		require.Equal(t, record.Body, body)
	}
	after, err := os.Stat(file)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "an identical shared hit must keep the original file")
	require.Equal(t, before.ModTime(), after.ModTime())
}

func TestIndexProviderCoarseMtimeDoesNotRewriteDisk(t *testing.T) {
	record := &HelmIndexRecord{Body: upstreamIndex(1), ETag: `"one"`, StoredAt: time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)}
	shared := &providerRecordCache{record: record}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	_, err := store.Revalidate(t.Context(), "view", regressionClient("https://unused.invalid"))
	require.NoError(t, err)
	stamp, err := time.Parse(time.RFC3339Nano, record.StoredAt)
	require.NoError(t, err)
	coarse := stamp.Truncate(time.Second)
	file := store.cacheFileName("view")
	require.NoError(t, os.Chtimes(file, coarse, coarse))
	before, err := os.Stat(file)
	require.NoError(t, err)
	for range 2 {
		body, err := store.Revalidate(t.Context(), "view", regressionClient("https://unused.invalid"))
		require.NoError(t, err)
		require.Equal(t, record.Body, body)
	}
	after, err := os.Stat(file)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "coarse timestamp precision must not hide a replaced cache file")
	require.Equal(t, before.ModTime(), after.ModTime())
	local, err := store.readRecord("view", false)
	require.NoError(t, err)
	require.Equal(t, coarse.Format(time.RFC3339Nano), local.StoredAt, "local age must still respect the earlier filesystem timestamp")
}

func TestIndexStoreUsesFreshRecordFoundAfterClaim(t *testing.T) {
	for _, diskFailure := range []bool{false, true} {
		t.Run(strconv.FormatBool(diskFailure), func(t *testing.T) {
			var requests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			t.Cleanup(origin.Close)
			old := &HelmIndexRecord{Body: upstreamIndex(1), Generation: "old", StoredAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
			next := &HelmIndexRecord{Body: upstreamIndex(2), Generation: "new", StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
			cache := &indexChangedBeforeClaimCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: old}}, next: next}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, cache)
			if diskFailure {
				require.NoError(t, os.Mkdir(store.cacheFileName("view"), 0o700))
			}
			body, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
			require.NoError(t, err)
			require.Equal(t, next.Body, body)
			require.Zero(t, requests.Load(), "a newly confirmed fresh record must not require a healthy origin")
			require.Empty(t, cache.owner, "the unconsumed claim must be released even after disk installation")
			require.Zero(t, cache.publishCalls)
			if !diskFailure {
				body, err = store.Get("view")
				require.NoError(t, err)
				require.Equal(t, next.Body, body)
			}
		})
	}
}

type indexReadsFailAfterClaim struct {
	*metadataProviderRecordCache
	claimed bool
}

func (c *indexReadsFailAfterClaim) BeginHelmIndexFetch(ctx context.Context, key string) (string, error) {
	c.claimed = true
	return c.atomicProviderRecordCache.BeginHelmIndexFetch(ctx, key)
}

func (c *indexReadsFailAfterClaim) GetHelmIndexMetadataContext(ctx context.Context, key string) (*HelmIndexRecord, error) {
	if c.claimed {
		return nil, context.DeadlineExceeded
	}
	return c.metadataProviderRecordCache.GetHelmIndexMetadataContext(ctx, key)
}

func TestIndexStoreDoesNotResurrectSupersededDiskAfterClaim(t *testing.T) {
	for _, firstOriginFailure := range []bool{false, true} {
		t.Run(strconv.FormatBool(firstOriginFailure), func(t *testing.T) {
			var headers recordedIndexHeaders
			var requests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers.record(r.Header.Get("If-None-Match"))
				if requests.Add(1) == 1 && firstOriginFailure {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(origin.Close)
			old := &HelmIndexRecord{Body: upstreamIndex(1), Generation: "old", ETag: `"old"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano), URLIdentity: IndexURLIdentity(origin.URL + "/index.yaml")}
			newer := &HelmIndexRecord{Body: upstreamIndex(2), Generation: "new", ETag: `"new"`, StoredAt: old.StoredAt, URLIdentity: old.URLIdentity}
			cache := &indexReadsFailAfterClaim{metadataProviderRecordCache: &metadataProviderRecordCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: newer}}, missingBody: true}}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, cache)
			require.True(t, store.installRecord("view", old))
			body, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
			wantHeaders := []string{""}
			if firstOriginFailure {
				require.Error(t, err)
				_, err = os.Stat(store.cacheFileName("view"))
				require.ErrorIs(t, err, os.ErrNotExist)
				body, err = store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
				wantHeaders = append(wantHeaders, "")
			}
			require.NoError(t, err)
			require.Equal(t, newer.Body, body)
			require.Equal(t, wantHeaders, headers.snapshot(), "known superseded bytes cannot become fresh after a timeout")
		})
	}
}

func TestIndexStoreChecksumBindsAllRecordMetadata(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.SetWithValidators("view", upstreamIndex(1), `"old"`, "original"))
	file := store.cacheFileName("view")
	original, err := os.ReadFile(file)
	require.NoError(t, err)
	for _, field := range []string{"generation", "etag", "lastModified", "urlIdentity", "storedAt"} {
		t.Run(field, func(t *testing.T) {
			var record map[string]any
			require.NoError(t, json.Unmarshal(original, &record))
			record[field] = "changed"
			if field == "storedAt" {
				record[field] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
			}
			raw, err := json.Marshal(record)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(file, raw, 0o600))
			_, err = store.GetRetained("view")
			require.Error(t, err)
		})
	}
	require.NoError(t, os.WriteFile(file, original, 0o600))
	_, err = store.GetRetained("view")
	require.NoError(t, err, "an unmodified record must retain conditional reuse")
}

func TestIndexStoreRetirementFailureKeepsSharedAndOriginUsable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires permission enforcement for the cache directory")
	}
	var requests atomic.Int32
	var headers recordedIndexHeaders
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		headers.record(r.Header.Get("If-None-Match"))
		_, _ = w.Write(upstreamIndex(3))
	}))
	t.Cleanup(origin.Close)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	old := &HelmIndexRecord{Body: upstreamIndex(1), ETag: `"old"`, StoredAt: stamp}
	newer := &HelmIndexRecord{Body: upstreamIndex(2), ETag: `"new"`, StoredAt: stamp}
	cache := &indexReadsFailAfterClaim{metadataProviderRecordCache: &metadataProviderRecordCache{atomicProviderRecordCache: &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{record: newer}}}}
	dir := t.TempDir()
	store := NewIndexStore(dir, time.Hour, 1<<20, cache)
	require.True(t, store.installRecord("view", old))
	require.NoError(t, os.WriteFile(store.cacheFileName("view")+".lock", nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0o700)) })
	body, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
	require.NoError(t, err)
	require.Equal(t, newer.Body, body)
	require.Zero(t, requests.Load())
	// Even after permissions recover, a later Redis outage cannot restore A.
	require.NoError(t, os.Chmod(dir, 0o700))
	cache.claimed = true
	cache.err = context.DeadlineExceeded
	body, err = store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
	require.NoError(t, err)
	require.Equal(t, upstreamIndex(3), body)
	require.EqualValues(t, 1, requests.Load())
	require.Equal(t, []string{""}, headers.snapshot())
	_, err = store.Get("view")
	require.Error(t, err, "disk reuse stays disabled until a new store is created")
}

func TestIndexStoreRejectsAlteredValidatorsDuringSharedReadFailure(t *testing.T) {
	var headers recordedIndexHeaders
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers.record(r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", `"new"`)
		if r.Header.Get("If-None-Match") == `"new"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(upstreamIndex(2))
	}))
	t.Cleanup(origin.Close)
	cache := &atomicProviderRecordCache{providerRecordCache: &providerRecordCache{err: io.ErrUnexpectedEOF}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, cache)
	record := &HelmIndexRecord{Body: upstreamIndex(1), ETag: `"old"`, StoredAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano), URLIdentity: IndexURLIdentity(origin.URL + "/index.yaml")}
	require.True(t, store.installRecord("view", record))
	file := store.cacheFileName("view")
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	var disk indexRecord
	require.NoError(t, json.Unmarshal(raw, &disk))
	disk.ETag = `"new"`
	raw, err = json.Marshal(disk)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, raw, 0o600))
	body, err := store.Revalidate(t.Context(), "view", regressionClient(origin.URL))
	require.NoError(t, err)
	require.Equal(t, upstreamIndex(2), body)
	require.Equal(t, []string{""}, headers.snapshot())
	require.Equal(t, 1, cache.publishCalls)
}

type indexReleaseDeadlineProbe struct{ deadline time.Time }

func (p *indexReleaseDeadlineProbe) ReleaseHelmIndexFetch(ctx context.Context, _, _ string) error {
	p.deadline, _ = ctx.Deadline()
	return nil
}

type indexShortDeadlineContext struct{ context.Context }

func (indexShortDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(4 * time.Millisecond), true
}

func TestIndexFetchReleaseRetainsMinimumBudget(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(strconv.FormatBool(canceled), func(t *testing.T) {
			var ctx context.Context = indexShortDeadlineContext{t.Context()}
			if canceled {
				parent, cancel := context.WithCancel(t.Context())
				cancel()
				ctx = parent
			} else {
				require.NoError(t, ctx.Err())
			}
			probe := &indexReleaseDeadlineProbe{}
			started := time.Now()
			releaseIndexFetch(ctx, probe, "view", "owner")
			require.GreaterOrEqual(t, probe.deadline.Sub(started), 25*time.Millisecond)
		})
	}
}

type indexRenewalProbe struct {
	calls   atomic.Int32
	current bool
}

func (p *indexRenewalProbe) RenewHelmIndexFetch(context.Context, string, string) (bool, error) {
	p.calls.Add(1)
	return p.current, nil
}

func TestIndexFetchRenewsOnlyDuringLiveOwnership(t *testing.T) {
	for _, current := range []bool{false, true} {
		t.Run(strconv.FormatBool(current), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				probe := &indexRenewalProbe{current: current}
				stop := keepIndexFetchAlive(t.Context(), probe, "view", "owner")
				time.Sleep(2 * IndexFetchLease)
				synctest.Wait()
				if current {
					require.GreaterOrEqual(t, probe.calls.Load(), int32(5))
				} else {
					require.EqualValues(t, 1, probe.calls.Load())
				}
				stop()
				synctest.Wait()
				calls := probe.calls.Load()
				time.Sleep(2 * IndexFetchLease)
				require.Equal(t, calls, probe.calls.Load())
			})
		})
	}
}
