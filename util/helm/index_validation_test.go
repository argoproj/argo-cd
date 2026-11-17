package helm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	require.ErrorContains(t, err, "unsupported apiVersion", "same-size changes must be validated even when timestamps are unchanged")
	_, err = store.GetRetained("view")
	require.ErrorContains(t, err, "unsupported apiVersion")
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
