package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

func newTestIndexStore(t *testing.T, expiration time.Duration, maxSize int64) (*IndexStore, string) {
	t.Helper()
	dir := t.TempDir()
	return NewIndexStore(dir, expiration, maxSize), dir
}

func sampleIndexData(name string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
entries:
  demo:
    - version: 1.0.0
      name: %s
      created: "2024-01-01T00:00:00Z"
      urls:
        - https://example.com/charts/demo-1.0.0.tgz
      digest: "deadbeef"
generated: "2024-01-01T00:00:00Z"
`, name))
}

func TestIndexStoreSetGetRoundTrip(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	require.NoError(t, store.Set("https://charts.example.com", data))

	got, err := store.Get("https://charts.example.com")
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestIndexStoreFreshnessExpiry(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	require.NoError(t, store.Set("https://charts.example.com", data))

	// Within freshness: Get succeeds.
	_, err := store.Get("https://charts.example.com")
	require.NoError(t, err)

	// Age the record past freshness: Get misses, GetRetained still serves.
	path := store.cacheFileName("https://charts.example.com")
	stale := time.Now().Add(-2 * time.Minute)
	require.NoError(t, os.Chtimes(path, stale, stale))

	_, err = store.Get("https://charts.example.com")
	require.Error(t, err, "fresh Get must miss past expiration")

	retained, err := store.GetRetained("https://charts.example.com")
	require.NoError(t, err)
	assert.Equal(t, data, retained)
}

func TestIndexStoreRecordDigestRejectsTampering(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	require.NoError(t, store.Set("https://charts.example.com", data))

	path := store.cacheFileName("https://charts.example.com")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	// Tamper with a header field covered by the record checksum.
	tampered := strings.Replace(string(raw), `"1"`, `"0"`, 1)
	if tampered == string(raw) {
		t.Skip("checksum-covered field not present in encoded record")
	}
	require.NoError(t, os.WriteFile(path, []byte(tampered), 0o600))

	_, err = store.Get("https://charts.example.com")
	assert.Error(t, err, "tampered record must be rejected")
}

func TestIndexStoreStateInvalidation(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	require.NoError(t, store.Set("https://charts.example.com", data))

	path := store.cacheFileName("https://charts.example.com")

	// Missing state file: disk eligibility unconfirmed.
	require.NoError(t, os.Remove(path+".state"))
	_, err := store.Get("https://charts.example.com")
	assert.Error(t, err, "record without state file must be rejected")
}

func TestIndexStoreOversizedRecordRejected(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 64)
	err := store.Set("https://charts.example.com", sampleIndexData("demo"))
	assert.Error(t, err, "record larger than the configured maximum must be rejected at Set time")
}

func TestIndexStoreUpdateValidatorsRefreshesRecord(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	require.NoError(t, store.SetWithValidators("https://charts.example.com", data, `"etag-1"`, "Mon, 01 Jan 2024 00:00:00 GMT"))

	require.NoError(t, store.UpdateValidators("https://charts.example.com", `"etag-2"`, "Tue, 02 Jan 2024 00:00:00 GMT"))

	etag, lastModified, err := store.GetHelmIndexValidators("https://charts.example.com")
	require.NoError(t, err)
	assert.Equal(t, `"etag-2"`, etag)
	assert.Equal(t, "Tue, 02 Jan 2024 00:00:00 GMT", lastModified)
}

func TestIndexStoreCopyToBothNames(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")
	destDir := t.TempDir()

	require.NoError(t, store.CopyTo("https://charts.example.com", data, destDir, "example"))
	// Registered name
	b, err := os.ReadFile(filepath.Join(destDir, cacheIndexFileName("example")))
	require.NoError(t, err)
	assert.Equal(t, data, b)
	// Manager name (installed separately by prewarmIndexes)
	managerName := managerKeyPrefix + managerKey("https://charts.example.com")
	require.NoError(t, store.CopyTo("https://charts.example.com", data, destDir, managerName))
	b, err = os.ReadFile(filepath.Join(destDir, cacheIndexFileName(managerName)))
	require.NoError(t, err)
	assert.Equal(t, data, b)

	// Copy again without replace: idempotent, keeps original bytes.
	replacement := sampleIndexData("other")
	require.NoError(t, store.CopyTo("https://charts.example.com", replacement, destDir, "example"))
	b, err = os.ReadFile(filepath.Join(destDir, cacheIndexFileName("example")))
	require.NoError(t, err)
	assert.Equal(t, data, b, "non-replacing copy must not overwrite")

	// Replace overwrites.
	require.NoError(t, store.copyTo("https://charts.example.com", replacement, destDir, "example", true))
	b, err = os.ReadFile(filepath.Join(destDir, cacheIndexFileName("example")))
	require.NoError(t, err)
	assert.Equal(t, replacement, b)
}

func TestIndexStoreSweepEvictsExpired(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	require.NoError(t, store.Set("https://charts.example.com", sampleIndexData("demo")))

	// retention() floors at 24h, so age the file past expiration+retention.
	path := store.cacheFileName("https://charts.example.com")
	old := time.Now().Add(-25 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	store.Sweep()

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "record past retention must be swept")
}

func TestIndexStoreInvalidBodyNotStored(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	err := store.Set("https://charts.example.com", []byte("not an index"))
	assert.Error(t, err, "invalid index body must be rejected at Set time")
}

func TestCacheFileNameIsStableHex(t *testing.T) {
	store, _ := newTestIndexStore(t, time.Minute, 1024)
	sum := sha256.Sum256([]byte("https://charts.example.com"))
	assert.Equal(t, filepath.Join(store.dir, hex.EncodeToString(sum[:])+".json"), store.cacheFileName("https://charts.example.com"))
}

// An index followed by an EMPTY YAML document must stay
// acceptable — the pinned Helm loader also stops at the first document.
// Regression coverage lives in index_validation_test.go (shared with PR1).

// countingMetaCache wraps an in-memory record store and counts how often the
// FULL record (body transfer) versus the small meta entry is read.
type countingMetaCache struct {
	mu        sync.Mutex
	record    *HelmIndexRecord
	meta      *HelmIndexMeta
	metaLive  bool
	fullReads int64
	metaReads int64
	metaBlock chan struct{} // non-nil blocks GetHelmIndexMeta until closed
}

func newCountingMetaCache() *countingMetaCache {
	return &countingMetaCache{}
}

func (c *countingMetaCache) publish(record *HelmIndexRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record = record
	c.meta = &HelmIndexMeta{BodyDigest: record.BodyDigest, ETag: record.ETag, LastModified: record.LastModified, URLIdentity: record.URLIdentity, Generation: record.Generation, StoredAt: record.StoredAt}
	c.metaLive = true
}

func (c *countingMetaCache) GetHelmIndexRetainedRecord(string) (*HelmIndexRecord, error) {
	c.fullReads++
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.record == nil {
		return nil, cacheutil.ErrCacheMiss
	}
	return c.record, nil
}

func (c *countingMetaCache) SetHelmIndexRetainedRecord(_ string, record *HelmIndexRecord) error {
	c.publish(record)
	return nil
}

func (c *countingMetaCache) GetHelmIndexMeta(string) (*HelmIndexMeta, error) {
	c.metaReads++
	if c.metaBlock != nil {
		<-c.metaBlock
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.metaLive || c.meta == nil {
		return nil, cacheutil.ErrCacheMiss
	}
	return c.meta, nil
}

func (c *countingMetaCache) dropMeta() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metaLive = false
}

func (c *countingMetaCache) swapMetaDigest(digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.meta = &HelmIndexMeta{BodyDigest: digest, ETag: c.meta.ETag, LastModified: c.meta.LastModified, URLIdentity: c.meta.URLIdentity, Generation: c.meta.Generation, StoredAt: c.meta.StoredAt}
}

func (s *IndexStore) digestOf(t *testing.T, repoURL string) string {
	t.Helper()
	record, err := s.readRecord(repoURL, false)
	require.NoError(t, err)
	return record.BodyDigest
}

func metaTestSetup(t *testing.T) (*IndexStore, *countingMetaCache, *nativeHelmChart, string) {
	t.Helper()
	cache := newCountingMetaCache()
	store := NewIndexStore(t.TempDir(), time.Minute, 1<<20, cache)
	// NewIndexStore wires metaCache automatically via the IndexMetaCache
	// type assertion; assert it so the test cannot silently regress.
	require.NotNil(t, store.metaCache, "counting cache must satisfy IndexMetaCache")
	const repoURL = "https://charts.example.com"
	client := NewClient(repoURL, HelmCreds{}, false, "", "", WithUserAgent("meta-test-agent")).(*nativeHelmChart)
	identity, err := client.indexURLIdentityContext(t.Context(), repoURL)
	require.NoError(t, err)
	body := sampleIndexData("demo")
	require.NoError(t, store.Set(repoURL, body))
	cache.publish(&HelmIndexRecord{
		Body:        body,
		BodyDigest:  store.digestOf(t, repoURL),
		URLIdentity: identity,
		StoredAt:    time.Now().UTC().Format(time.RFC3339Nano),
	})
	return store, cache, client, repoURL
}

func TestMetaFastPathAvoidsFullRecordRead(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)

	// Warm build through revalidate: local body verified, shared meta matches.
	body, refreshed, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.False(t, refreshed)
	assert.EqualValues(t, 0, cache.fullReads, "warm build must not read the full shared record")
	assert.EqualValues(t, 1, cache.metaReads, "warm build reads only the small meta entry")
}

func TestMetaFastPathFallsBackOnMissingMeta(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)
	cache.dropMeta()

	body, _, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.EqualValues(t, 1, cache.fullReads, "missing meta must fall back to the full record")
}

func TestMetaFastPathFallsBackOnDigestMismatch(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)
	cache.swapMetaDigest("deadbeef")

	body, _, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.EqualValues(t, 1, cache.fullReads, "digest mismatch must fall back to the full record")
}

// TestStaleMetaPolicy documents the explicit freshness contract for partial
// publication: when the shared record holds body B but the meta entry still
// describes body A (old writer, failed meta write), a reader holding local A
// serves A from the matching meta. The divergence window is bounded by the
// freshness window — every successful publication rewrites meta together
// with the record (one retry on transient failure), and readers whose local
// body does NOT match meta take the full-record fallback and see B
// immediately.
func TestStaleMetaPolicy(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)

	// Simulate partial publication: record advances to B (genuinely
	// different bytes), meta still describes A.
	bodyA := sampleIndexData("demo")
	bodyB := sampleIndexData("other")
	require.NotEqual(t, string(bodyA), string(bodyB), "bodies must differ")
	cache.mu.Lock()
	cache.record = &HelmIndexRecord{
		Body:        bodyB,
		BodyDigest:  sha256Hex(bodyB),
		ETag:        `"etag-b"`,
		URLIdentity: cache.record.URLIdentity, // same request identity
		StoredAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
	cache.mu.Unlock()

	// Reader whose local body matches meta A: serves A from the matching
	// meta (explicit policy; no full read) — while meta A is still fresh.
	body, _, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)
	assert.Equal(t, string(bodyA), string(body), "matching meta authorizes the verified local body")
	assert.EqualValues(t, 0, cache.fullReads, "matching meta authorizes the verified local body")

	// After meta A's freshness expires, the same reader may no longer serve
	// A on meta authority: the fresh check rejects the aged StoredAt and the
	// full-record fallback runs, observing B.
	cache.mu.Lock()
	agedMeta := *cache.meta
	agedMeta.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	cache.meta = &agedMeta
	cache.mu.Unlock()
	agedBody, _, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)
	assert.Equal(t, string(bodyB), string(agedBody), "expired meta must not serve the old body; the full record (B) wins")
	assert.EqualValues(t, 1, cache.fullReads, "expired meta forces the full-record fallback")

	// A reader whose local body matches NEITHER A nor B takes the full
	// fallback and observes the new record B even while meta is fresh. B is
	// republished with a fresh timestamp so it is newer than this reader's
	// local record — otherwise the never-regress guard correctly keeps the
	// locally newer record.
	otherStore, otherCache, otherClient, otherURL := metaTestSetup(t)
	cache.mu.Lock()
	refreshedB := *cache.record
	refreshedB.StoredAt = time.Now().UTC().Format(time.RFC3339Nano)
	cache.mu.Unlock()
	otherCache.mu.Lock()
	otherCache.record = &refreshedB   // shared record now B, newer than local
	otherCache.meta = &HelmIndexMeta{ // meta describing a third body
		BodyDigest: sha256Hex(sampleIndexData("unrelated")),
		StoredAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	otherCache.mu.Unlock()
	sharedBodyB, _, otherErr := otherStore.revalidate(t.Context(), otherURL, otherClient, false)
	require.NoError(t, otherErr)
	assert.Equal(t, string(bodyB), string(sharedBodyB))
	assert.EqualValues(t, 1, otherCache.fullReads, "non-matching meta must fall back to the full record")
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestMetaFastPathNeverMixesValidators(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)

	// Meta carries validators the local record never saw; a matching meta
	// must transfer them wholesale (never mixing them with other bodies).
	cache.mu.Lock()
	cache.meta.ETag = `"shared-etag"`
	cache.meta.LastModified = "Wed, 01 Jan 2025 00:00:00 GMT"
	cache.meta.StoredAt = time.Now().UTC().Format(time.RFC3339Nano)
	cache.mu.Unlock()

	_, _, err := store.revalidate(t.Context(), repoURL, client, false)
	require.NoError(t, err)

	etag, modified, err := store.GetHelmIndexValidators(repoURL)
	require.NoError(t, err)
	assert.Equal(t, `"shared-etag"`, etag, "validators must come from the matching meta")
	assert.Equal(t, "Wed, 01 Jan 2025 00:00:00 GMT", modified)
}

// TestMetaFastPathRespectsCancellationBudget reproduces the deadline
// regression: a metadata read that ignores the request budget blocked a warm
// operation past a 250 ms deadline. The read must be bounded by the shared
// budget (min(2s, remaining/4)) and release the request to the origin path.
func TestMetaFastPathRespectsCancellationBudget(t *testing.T) {
	store, cache, client, repoURL := metaTestSetup(t)

	// Block the meta read indefinitely (simulates a stalled backend).
	blocked := make(chan struct{})
	cache.metaBlock = blocked
	t.Cleanup(func() { close(blocked) })

	start := time.Now()
	// 250ms deadline: the shared budget (min(2s, remaining/4)) must bound the
	// meta read to ~62ms and let revalidate proceed (fallback path) before
	// the caller's deadline expires.
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := store.revalidate(ctx, repoURL, client, false)
		done <- err
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "revalidate blocked on the metadata read past the shared budget")
	}
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 1500*time.Millisecond, "metadata read must be bounded, took %v", elapsed)
}
