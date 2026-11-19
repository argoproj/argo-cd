package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, data, []byte(got))
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
	assert.Error(t, err, "fresh Get must miss past expiration")

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
