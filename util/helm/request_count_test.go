package helm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestCountFixture is the automated version of the manual verification from
// the investigation: it asserts exactly how many origin requests a
// DependencyBuild-style index lookup costs in each store state.
//
//	warm store            → 0 requests (served entirely from disk)
//	expired + validators  → 1 request (conditional GET, 304)
//	expired, no validators→ 1 request (full GET, 200)
//	no entry              → 1 request (full GET, 200)
func TestIndexStoreRequestCounts(t *testing.T) {
	indexBody := "apiVersion: v1\nentries: {}\n"

	setup := func(t *testing.T, serve304 bool) (*httptest.Server, *atomic.Int32) {
		t.Helper()
		var requests atomic.Int32
		// The origin bumps the index content on every full GET (v1, v2, ...).
		// A client that correctly sends If-None-Match gets a 304 and keeps
		// its cached v1 body; a client that fails to condition gets a fresh
		// 200 with different content — the assertion on the returned bytes
		// catches that.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := requests.Add(1)
			if serve304 && r.Header.Get("If-None-Match") == `"idx-v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			if serve304 {
				w.Header().Set("ETag", fmt.Sprintf(`"idx-v%d"`, n))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s#gen-%d\n", indexBody, n)
		}))
		t.Cleanup(srv.Close)
		return srv, &requests
	}

	newChartClient := func(srvURL string) *nativeHelmChart {
		return NewClientWithLock(srvURL, HelmCreds{}, globalLock, false, "", "").(*nativeHelmChart)
	}

	t.Run("warm store costs zero requests", func(t *testing.T) {
		srv, requests := setup(t, true)
		store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
		identity := "https://example.com\x00fp"
		require.NoError(t, store.Set(identity, []byte(indexBody)))

		data, err := store.Revalidate(context.Background(), identity, newChartClient(srv.URL))
		require.NoError(t, err)
		assert.Equal(t, indexBody, string(data))
		assert.Equal(t, int32(0), requests.Load(), "warm entry must be served from disk without touching the origin")
	})

	t.Run("expired entry with validators costs one 304", func(t *testing.T) {
		srv, requests := setup(t, true)
		store := NewIndexStore(t.TempDir(), 10*time.Millisecond, 1<<30)
		identity := "https://example.com\x00fp"
		client := newChartClient(srv.URL)

		// First revalidation: store has no validators yet → one full GET
		// (200 gen-1); body + ETag land in the store's versioned record.
		gen1, err := store.Revalidate(context.Background(), identity, client)
		require.NoError(t, err)
		require.Equal(t, int32(1), requests.Load())
		etag, _, err := store.GetHelmIndexValidators(identity)
		require.NoError(t, err)
		require.Equal(t, `"idx-v1"`, etag)

		// Expire, then revalidate: the fixture answers 304 ONLY on a matching
		// If-None-Match; a header-less request would return a new body
		// (gen-2), which the byte assertion below would catch.
		time.Sleep(30 * time.Millisecond)
		data, err := store.Revalidate(context.Background(), identity, client)
		require.NoError(t, err)
		assert.Equal(t, string(gen1), string(data), "304 must serve the retained body, not a fresh generation")
		assert.Equal(t, int32(2), requests.Load(), "revalidation must cost exactly one conditional origin request")

		// The 304 restarts freshness: a further lookup costs nothing again.
		_, err = store.Revalidate(context.Background(), identity, client)
		require.NoError(t, err)
		assert.Equal(t, int32(2), requests.Load(), "entry refreshed by 304 must be warm again")
	})

	t.Run("expired entry without validators costs one full GET", func(t *testing.T) {
		srv, requests := setup(t, false)
		store := NewIndexStore(t.TempDir(), 10*time.Millisecond, 1<<30)
		identity := "https://example.com\x00fp"

		_, err := store.Revalidate(context.Background(), identity, newChartClient(srv.URL))
		require.NoError(t, err)
		time.Sleep(30 * time.Millisecond)
		data, err := store.Revalidate(context.Background(), identity, newChartClient(srv.URL))
		require.NoError(t, err)
		// No validators → the origin always answers a full 200 with a new
		// generation; assert we got a complete (non-empty) replacement.
		assert.Contains(t, string(data), "#gen-2")
		assert.Equal(t, int32(2), requests.Load(), "validator-less origin: revalidation is one full GET")
	})

	t.Run("missing entry costs one full GET", func(t *testing.T) {
		srv, requests := setup(t, false)
		store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)

		_, err := store.Revalidate(context.Background(), "https://example.com\x00fp", newChartClient(srv.URL))
		require.NoError(t, err)
		assert.Equal(t, int32(1), requests.Load())
	})
}
