package helm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

type fakeIndex struct {
	APIVersion string                 `yaml:"apiVersion"`
	Entries    map[string]fakeEntries `yaml:"entries"`
}

type fakeEntry struct {
	Version string `yaml:"version"`
}

type fakeEntries []fakeEntry

// newIndexBody emits a realistic index.yaml in the shape the pinned Helm
// client produces: apiVersion v1, lowercased keys, string versions.
func newIndexBody(version string) []byte {
	index := fakeIndex{APIVersion: "v1", Entries: map[string]fakeEntries{"fake": []fakeEntry{{Version: version}}}}
	data, err := yaml.Marshal(index)
	if err != nil {
		panic(err)
	}
	return data
}

type recordingIndexCache struct {
	data       []byte
	sets       atomic.Int64
	gets       atomic.Int64
	recordSets atomic.Int64
}

func (f *recordingIndexCache) SetHelmIndex(_ string, indexData []byte) error {
	f.sets.Add(1)
	f.data = indexData
	return nil
}

func (f *recordingIndexCache) GetHelmIndex(_ string, indexData *[]byte) error {
	f.gets.Add(1)
	*indexData = f.data
	return nil
}

// recordIndexCache mimics the repo-server record contract: one atomic record
// carrying body + validators + a freshness timestamp. Freshness follows the
// same semantics as reposerver/cache: a record is fresh while now-StoredAt
// stays inside the expiration window.
type recordIndexCache struct {
	mu         sync.Mutex
	record     *HelmIndexRecord
	expiration time.Duration // <=0: always stale
	recordSets atomic.Int64
	recordGets atomic.Int64
}

func (f *recordIndexCache) SetHelmIndexRecord(_ string, indexData []byte, etag string, lastModified string) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	digest := sha256.Sum256(indexData)
	f.record = &HelmIndexRecord{Body: bytes.Clone(indexData), BodyDigest: hex.EncodeToString(digest[:]), ETag: etag, LastModified: lastModified, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return nil
}

func (f *recordIndexCache) SetHelmIndexRetainedRecord(_ string, record *HelmIndexRecord) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := *record
	if record.BodyDigest == "" {
		digest := sha256.Sum256(record.Body)
		clone.BodyDigest = hex.EncodeToString(digest[:])
	}
	f.record = &clone
	return nil
}

func (f *recordIndexCache) GetHelmIndexRetainedRecord(_ string) (*HelmIndexRecord, error) {
	f.recordGets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.record == nil {
		return nil, cacheutil.ErrCacheMiss
	}
	if !f.record.Fresh(f.expiration) {
		// Return the record anyway: the caller revalidates stale records with
		// the origin instead of discarding them.
		return f.record, nil
	}
	return f.record, nil
}

func (f *recordIndexCache) SetHelmIndex(_ string, indexData []byte) error {
	f.recordSets.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record = &HelmIndexRecord{Body: bytes.Clone(indexData), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return nil
}

func (f *recordIndexCache) GetHelmIndex(_ string, indexData *[]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Mirror reposerver semantics: a raw-body read only succeeds for a FRESH
	// record. A stale record must not be served without revalidation, so the
	// raw view misses and the client falls back to the conditional GET path.
	if f.record == nil || !f.record.Fresh(f.expiration) {
		*indexData = nil
		return cacheutil.ErrCacheMiss
	}
	*indexData = f.record.Body
	return nil
}

func TestIndexConditionalRevalidation(t *testing.T) {
	originRequests := atomic.Int64{}
	etag := `"index-etag-v1"`
	body := newIndexBody("1.0.0")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originRequests.Add(1)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(body)
	}))

	t.Run("SecondFetchIsConditionalAndReusesCachedBody", func(t *testing.T) {
		// A record-backed cache with a STALE record forces the client to
		// revalidate with the origin instead of trusting the cached bytes.
		cacheHeld := &recordIndexCache{expiration: time.Minute}
		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

		index, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		require.NotEmpty(t, index.Entries["fake"], "index must decode with entries")
		assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)
		firstCount := originRequests.Load()
		require.Equal(t, int64(1), firstCount)
		require.Equal(t, int64(1), cacheHeld.recordSets.Load(), "first fetch must publish the record")

		// Age the record past the freshness window so the next fetch must
		// revalidate. Failing-before: the client either re-downloads the
		// whole index (no conditional GET) or trusts stale bytes outright.
		cacheHeld.mu.Lock()
		stale := *cacheHeld.record
		stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
		cacheHeld.record = &stale
		cacheHeld.mu.Unlock()

		index2, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", index2.Entries["fake"][0].Version)
		assert.Equal(t, firstCount+1, originRequests.Load(), "second fetch must revalidate with exactly one conditional GET")
		assert.Equal(t, int64(1), cacheHeld.recordSets.Load(), "a 304 must not republish a new record generation")
	})

	t.Run("ChangedIndexIsRefetched", func(t *testing.T) {
		newBody := newIndexBody("2.0.0")
		server2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("If-None-Match") == etag {
				// Simulate the origin having a newer representation than the
				// cached one: a 200 with a new ETag, not a 304.
				w.Header().Set("ETag", `"index-etag-v2"`)
				_, _ = w.Write(newBody)
				return
			}
			w.Header().Set("ETag", etag)
			_, _ = w.Write(body)
		}))
		cacheHeld := &recordIndexCache{expiration: time.Minute}
		client := NewClient(server2.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

		index, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

		cacheHeld.mu.Lock()
		stale := *cacheHeld.record
		stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
		cacheHeld.record = &stale
		cacheHeld.mu.Unlock()

		index2, err := client.GetIndex(t.Context(), false, 1024*1024)
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", index2.Entries["fake"][0].Version)
		assert.Equal(t, int64(2), cacheHeld.recordSets.Load(), "changed index must republish")
	})
}

func TestIndexCredentialScopeIsolation(t *testing.T) {
	const user = "secret-user"
	const pass = "secret-pass"
	body := newIndexBody("1.0.0")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username != "" {
			if username != user || password != pass {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("ETag", `"scoped-etag"`)
		_, _ = w.Write(body)
	}))

	privateCache := &recordingIndexCache{}
	anonClient := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCacheScopedByCreds(privateCache))
	_, err := anonClient.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)

	// A different credential scope must not consume the anonymous entry and,
	// on publication, must not overwrite it either.
	authedCache := &recordingIndexCache{}
	authedClient := NewClient(server.URL, HelmCreds{Username: user, Password: pass, InsecureSkipVerify: true}, false, "", "", WithIndexCacheScopedByCreds(authedCache))
	index, err := authedClient.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

	// The two scopes must have used different cache keys.
	anonKey, err := (&nativeHelmChart{repoURL: server.URL, scopedByCreds: true}).cacheRepoKeyScoped(t.Context(), "")
	require.NoError(t, err)
	authedKey, err := (&nativeHelmChart{repoURL: server.URL, scopedByCreds: true}).cacheRepoKeyScoped(t.Context(), credsScope(HelmCreds{Username: user, Password: pass}))
	require.NoError(t, err)
	assert.NotEqual(t, anonKey, authedKey)
	assert.NotContains(t, anonKey, pass)
	assert.NotContains(t, authedKey, pass)
}

func TestIndexBadBodyIsNotCached(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"bad-index"`)
		_, _ = w.Write([]byte("this is not a helm index"))
	}))
	cacheHeld := &recordingIndexCache{}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.Error(t, err, "format validation must reject non-index bodies")
	assert.Equal(t, int64(0), cacheHeld.sets.Load(), "invalid index bodies must never be published")
}

func TestIndexUnusable304RetriesUnconditionally(t *testing.T) {
	requests := atomic.Int64{}
	// The origin answers the conditional request with an unusable 304 (no
	// matching ETag semantics): the client must fall back to a full GET.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			w.Header().Set("ETag", `"etag"`)
			_, _ = w.Write(newIndexBody("1.0.0"))
			return
		}
		if r.Header.Get("If-None-Match") != "" {
			// Malicious/incorrect 304: its ETag does not match the cached
			// representation the validator was sent for, so the response
			// validates nothing the client asked about. A bare 304 (no ETag
			// header) would be an "omitted" validator and stays usable.
			w.Header().Set("ETag", `"mismatched-etag"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"etag-%d"`, n))
		_, _ = w.Write(newIndexBody("2.0.0"))
	}))
	cacheHeld := &recordIndexCache{expiration: time.Minute}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	index, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", index.Entries["fake"][0].Version)

	cacheHeld.mu.Lock()
	stale := *cacheHeld.record
	stale.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	cacheHeld.record = &stale
	cacheHeld.mu.Unlock()

	index2, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index2.Entries["fake"][0].Version, "an unusable 304 must be retried unconditionally")
	assert.Equal(t, int64(3), requests.Load(), "expected: initial GET + unusable 304 + unconditional retry")
}

func TestIndexSplitCacheUsesBoundValidatorsOnly(t *testing.T) {
	// Validators bound in-process to one body must never 304-validate a
	// different body that reached the cache through another writer.
	etag := `"split-etag"`
	bodies := map[string][]byte{
		"1": newIndexBody("1.0.0"),
		"2": newIndexBody("2.0.0"),
	}
	var served atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := served.Add(1)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		if n <= 2 {
			_, _ = w.Write(bodies["1"])
			return
		}
		_, _ = w.Write(bodies["2"])
	}))
	cacheHeld := &recordIndexCache{expiration: time.Minute}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)

	// Replace the cache contents out-of-band (another replica published a
	// different body under the same key).
	cacheHeld.mu.Lock()
	replaced := *cacheHeld.record
	replaced.Body = bytes.Clone(bodies["2"])
	replaced.BodyDigest = ""
	replaced.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	cacheHeld.record = &replaced
	cacheHeld.mu.Unlock()

	// The stale-bound validators must not be paired with the new bytes.
	_, err = client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	index, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["fake"][0].Version)
	assert.Greater(t, served.Load(), int64(2), "validators bound to the old body must not 304-validate the new body")
}

func TestIndexRecordRoundTripThroughClient(t *testing.T) {
	// With a full record store (atomic body+validators), the client must
	// publish one record on the 200 and zero records on the 304.
	etag := `"record-etag"`
	origin := atomic.Int64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := origin.Add(1)
		if n > 1 && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write(newIndexBody("1.0.0"))
	}))
	cacheHeld := &recordingIndexCache{}
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cacheHeld))

	_, err := client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	require.Equal(t, int64(1), cacheHeld.sets.Load())
	_, err = client.GetIndex(t.Context(), false, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cacheHeld.sets.Load(), "304 must not republish")
	assert.Less(t, origin.Load(), int64(3), "second fetch should be a single conditional request")
	_ = time.Now // silence unused import if helpers change
}
