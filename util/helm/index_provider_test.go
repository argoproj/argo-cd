package helm

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
		return nil, errors.New("cache miss")
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

func TestIndexProviderSelectsNewestValidation(t *testing.T) {
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
			want := regressionIndexBody + "# disk\n"
			wantTime := diskTime
			if sharedNewer {
				sharedTime = diskTime.Add(10 * time.Minute)
				want = regressionIndexBody + "# Redis\n"
				wantTime = sharedTime
			}
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
			require.Equal(t, wantTime.Format(time.RFC3339Nano), record.StoredAt, "copying must not restart freshness")
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
		w.Header().Set("ETag", `W/"two"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	shared := &providerRecordCache{err: errors.New("Redis unavailable")}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20, shared)
	old := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	require.NoError(t, store.publishRecord("view", &indexRecord{APIVersion: "v1", ETag: `"one"`, Body: base64.StdEncoding.EncodeToString([]byte(regressionIndexBody)), StoredAt: old}))
	body, err := store.Revalidate(t.Context(), "view", NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart))
	require.NoError(t, err)
	require.Equal(t, regressionIndexBody, string(body))
	require.EqualValues(t, 1, requests.Load())
	etag, _, err := store.GetHelmIndexValidators("view")
	require.NoError(t, err)
	require.Equal(t, `W/"two"`, etag, "304 response metadata must replace validators when supplied")
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
	store := NewIndexStore(dir, time.Hour, 1<<20, &providerRecordCache{err: errors.New("Redis unavailable")})
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
	t.Cleanup(resume)
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
	_, err := client.GetIndex(ctx, true, 1<<20)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualValues(t, 1, requests.Load())
	resume()
	require.NoError(t, <-completed)
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
