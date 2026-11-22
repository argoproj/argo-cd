package helm

import (
	"sync"
	"testing"
	"time"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingMetaCache wraps an in-memory record store and counts how often the
// FULL record (body transfer) versus the small meta entry is read.
type countingMetaCache struct {
	mu        sync.Mutex
	record    *HelmIndexRecord
	meta      *HelmIndexMeta
	metaLive  bool
	fullReads int64
	metaReads int64
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

func newMetaTestIndex(t *testing.T, store *IndexStore, cache *countingMetaCache, version string) {
	t.Helper()
	body := sampleIndexData("demo")
	require.NoError(t, store.Set("https://charts.example.com", body))
	cache.publish(&HelmIndexRecord{
		Body:       body,
		BodyDigest: store.digestOf(t, "https://charts.example.com"),
		StoredAt:   time.Now().UTC().Format(time.RFC3339Nano),
	})
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
