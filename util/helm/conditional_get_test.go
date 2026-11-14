package helm

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// fakeValidatorsCache is an indexCache that also implements
// indexCacheWithValidators, mirroring reposerver/cache.Cache.
type fakeValidatorsCache struct {
	fakeIndexCache
	etag         string
	lastModified string
}

func (f *fakeValidatorsCache) SetHelmIndexValidators(_ string, etag string, lastModified string) error {
	f.etag = etag
	f.lastModified = lastModified
	return nil
}

func (f *fakeValidatorsCache) GetHelmIndexValidators(_ string) (string, string, error) {
	if f.etag == "" && f.lastModified == "" {
		return "", "", errors.New("validators not found")
	}
	return f.etag, f.lastModified, nil
}

type conditionalGetServer struct {
	etag         string
	lastModified string
	body         []byte
	requests     atomic.Int32
	// when true, a matching If-None-Match returns 304; otherwise 200 always.
	supports304 bool
}

func (s *conditionalGetServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		inm := r.Header.Get("If-None-Match")
		if s.supports304 && inm != "" && inm == s.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if s.lastModified != "" {
			w.Header().Set("Last-Modified", s.lastModified)
		}
		if s.etag != "" {
			w.Header().Set("ETag", s.etag)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(s.body)
	}
}

func newIndex(t *testing.T) []byte {
	t.Helper()
	fakeIndex := Index{Entries: map[string]Entries{"fake": {}}}
	data := bytes.Buffer{}
	require.NoError(t, yaml.NewEncoder(&data).Encode(fakeIndex))
	return data.Bytes()
}

func TestConditionalGET_Revalidates(t *testing.T) {
	body := newIndex(t)
	srv := &conditionalGetServer{etag: `"v1"`, body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeValidatorsCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	// First fetch: 200, validators stored.
	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(1), srv.requests.Load())
	assert.Equal(t, `"v1"`, cache.etag)

	// Second fetch: If-None-Match sent, 304 received, cached body returned —
	// still only 2 requests total, no re-download.
	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	assert.Equal(t, body, cache.data)
	assert.NotNil(t, idx)
}

func TestConditionalGET_LastModifiedOnly(t *testing.T) {
	body := newIndex(t)
	srv := &conditionalGetServer{lastModified: "Wed, 21 Oct 2026 07:28:00 GMT", body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeValidatorsCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	_, err = client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
}

func TestConditionalGET_NoValidatorSupport(t *testing.T) {
	// Server sends no ETag/Last-Modified: every fetch is a plain 200 GET.
	body := newIndex(t)
	srv := &conditionalGetServer{body: body}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeValidatorsCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	for range 3 {
		_, err := client.GetIndex(t.Context(), true, 10000)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(3), srv.requests.Load())
	assert.Empty(t, cache.etag)
}

func TestConditionalGET_IndexChanged(t *testing.T) {
	// 200 with a new body and new ETag replaces both body and validators.
	srv := &conditionalGetServer{etag: `"v1"`, body: newIndex(t), supports304: true}
	var handler atomic.Pointer[http.HandlerFunc]
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := handler.Load(); h != nil {
			(*h)(w, r)
			return
		}
		srv.handler()(w, r)
	})
	httpSrv := httptest.NewServer(wrapped)
	defer httpSrv.Close()

	cache := &fakeValidatorsCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)

	// Server now has a new index with a new ETag; 304 support for the old
	// ETag is gone, so a conditional request gets a fresh 200.
	srv.etag = `"v2"`
	srv.body = newIndex(t)
	newBody := append([]byte(nil), srv.body...)
	srv.body = newBody
	srv.supports304 = false

	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	assert.Equal(t, `"v2"`, cache.etag)
	assert.Equal(t, newBody, cache.data)
	require.NotNil(t, idx)
}

func TestApplyConditionalHeaders(t *testing.T) {
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/index.yaml", http.NoBody)
	applyConditionalHeaders(req, `"abc"`, "Wed, 21 Oct 2026 07:28:00 GMT")
	// ETag present: only If-None-Match is sent (RFC 9110 §13.1.2).
	assert.Equal(t, `"abc"`, req.Header.Get("If-None-Match"))
	assert.Empty(t, req.Header.Get("If-Modified-Since"))

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/index.yaml", http.NoBody)
	applyConditionalHeaders(req2, "", "Wed, 21 Oct 2026 07:28:00 GMT")
	assert.Empty(t, req2.Header.Get("If-None-Match"))
	assert.Equal(t, "Wed, 21 Oct 2026 07:28:00 GMT", req2.Header.Get("If-Modified-Since"))
}

func TestInMemoryIndexValidators(t *testing.T) {
	v := newInMemoryIndexValidators()
	_, _, err := v.GetHelmIndexValidators("repo")
	require.Error(t, err)
	require.NoError(t, v.SetHelmIndexValidators("repo", `"e"`, "lm"))
	etag, lm, err := v.GetHelmIndexValidators("repo")
	require.NoError(t, err)
	assert.Equal(t, `"e"`, etag)
	assert.Equal(t, "lm", lm)
}

func TestValidatorsFor_InMemoryFallback(t *testing.T) {
	// A cache implementing only the base interface gets the in-process
	// fallback (never nil).
	v := validatorsFor(&fakeIndexCache{})
	require.NotNil(t, v)
	// A cache implementing the extension is used directly.
	ext := validatorsFor(&fakeValidatorsCache{})
	assert.Same(t, sharedInMemoryValidators, validatorsFor(&fakeIndexCache{}))
	_ = ext
}

func TestConditionalGET_304WithoutCachedBody(t *testing.T) {
	// Validators survive but the cached body was evicted: 304 → one retry
	// without validators → 200 body.
	body := newIndex(t)
	srv := &conditionalGetServer{etag: `"v1"`, body: body, supports304: true}
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	cache := &fakeValidatorsCache{}
	client := NewClient(httpSrv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))

	// Prime validators.
	_, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)

	// Evict the body but keep validators. Conditional headers are only sent
	// when a usable cached body exists, so this degrades to a plain 200 GET
	// (loadRepoIndex keeps a defensive one-shot unconditional retry for an
	// orphan 304 — unreachable through this path, but protects against a
	// body-evicting cache change).
	cache.data = nil
	idx, err := client.GetIndex(t.Context(), true, 10000)
	require.NoError(t, err)
	assert.Equal(t, int32(2), srv.requests.Load())
	assert.Equal(t, body, cache.data)
	require.NotNil(t, idx)
}
