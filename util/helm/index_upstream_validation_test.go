package helm

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func upstreamIndex(version int32) []byte {
	return []byte(fmt.Sprintf("apiVersion: v1\nentries:\n  child:\n    - version: %d.0.0\n      urls: [child-%d.0.0.tgz]\n", version, version))
}

func TestIndexUpstreamValidatorChanges(t *testing.T) {
	for _, mode := range []string{"etag", "weak etag", "last modified", "both", "none"} {
		t.Run(mode, func(t *testing.T) {
			var version atomic.Int32
			version.Store(1)
			type request struct {
				etag, modified string
				status         int
			}
			requests := make(chan request, 8)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				generation := version.Load()
				etag, modified := "", ""
				if mode == "etag" || mode == "both" || mode == "weak etag" {
					etag = fmt.Sprintf(`"v%d"`, generation)
					if mode == "weak etag" {
						etag = "W/" + etag
					}
					w.Header().Set("ETag", etag)
				}
				if mode == "last modified" || mode == "both" {
					modified = time.Date(2026, 1, 1, int(generation), 0, 0, 0, time.UTC).Format(http.TimeFormat)
					w.Header().Set("Last-Modified", modified)
				}
				status := http.StatusOK
				if etag != "" && r.Header.Get("If-None-Match") == etag ||
					etag == "" && modified != "" && r.Header.Get("If-Modified-Since") == modified {
					status = http.StatusNotModified
				}
				requests <- request{r.Header.Get("If-None-Match"), r.Header.Get("If-Modified-Since"), status}
				w.WriteHeader(status)
				if status == http.StatusOK {
					_, _ = w.Write(upstreamIndex(generation))
				}
			}))
			t.Cleanup(server.Close)
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			client := regressionClient(server.URL)
			fetch := func(want int32) {
				t.Helper()
				body, err := store.Revalidate(t.Context(), "upstream", client)
				require.NoError(t, err)
				require.Equal(t, upstreamIndex(want), body)
			}
			fetch(1)
			require.Equal(t, http.StatusOK, (<-requests).status)
			fetch(1)
			require.Empty(t, requests, "a warm read must not check upstream")
			regressionExpire(t, store, "upstream")
			fetch(1)
			unchanged := <-requests
			wantStatus := http.StatusNotModified
			if mode == "none" {
				wantStatus = http.StatusOK
			}
			require.Equal(t, wantStatus, unchanged.status)
			if mode == "last modified" {
				require.NotEmpty(t, unchanged.modified)
				require.Empty(t, unchanged.etag)
			}
			if mode == "both" {
				require.NotEmpty(t, unchanged.etag)
				require.Empty(t, unchanged.modified, "ETag takes precedence")
			}
			version.Store(2)
			regressionExpire(t, store, "upstream")
			fetch(2)
			changed := <-requests
			require.Equal(t, http.StatusOK, changed.status)
			require.Equal(t, unchanged.etag, changed.etag)
			require.Equal(t, unchanged.modified, changed.modified)
			regressionExpire(t, store, "upstream")
			fetch(2)
			revalidated := <-requests
			require.Equal(t, wantStatus, revalidated.status)
			if mode != "none" {
				require.NotEqual(t, unchanged, revalidated, "the new response's validators must replace the old generation")
			}
		})
	}
}

func TestIndexUpstreamOrphan304RetryIsBounded(t *testing.T) {
	for _, repeat304 := range []bool{false, true} {
		t.Run(fmt.Sprintf("repeat304=%t", repeat304), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if requests.Add(1) == 1 || repeat304 {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(server.Close)
			body, err := regressionClient(server.URL).loadRepoIndex(t.Context(), 1<<20)
			if repeat304 {
				require.ErrorContains(t, err, "304")
				require.Empty(t, body)
			} else {
				require.NoError(t, err)
				require.Equal(t, upstreamIndex(2), body)
			}
			require.EqualValues(t, 2, requests.Load(), "an unusable 304 must retry exactly once")
		})
	}
}

func TestIndexUpstreamRedirectRetainsConditionalRequest(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			http.Redirect(w, r, "/actual/index.yaml", http.StatusTemporaryRedirect)
			return
		}
		requests <- r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `"redirected"`)
		if r.Header.Get("If-None-Match") == `"redirected"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(upstreamIndex(1))
	}))
	t.Cleanup(server.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		body, err := store.Revalidate(t.Context(), "redirect", regressionClient(server.URL))
		require.NoError(t, err)
		require.Equal(t, upstreamIndex(1), body)
		regressionExpire(t, store, "redirect")
	}
	require.Empty(t, <-requests)
	require.Equal(t, `"redirected"`, <-requests)
}

func TestIndexUpstreamValidationThroughProxy(t *testing.T) {
	for _, policy := range []string{"expired", "always fresh", "forced"} {
		t.Run(policy, func(t *testing.T) {
			var originRequests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				originRequests.Add(1)
				w.Header().Set("ETag", `"new"`)
				_, _ = w.Write(upstreamIndex(2))
			}))
			t.Cleanup(origin.Close)
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// A proxy may satisfy an unconditional request using its own
				// still-fresh response unless origin validation is requested.
				if !strings.Contains(r.Header.Get("Cache-Control"), "no-cache") {
					w.Header().Set("ETag", `"old"`)
					_, _ = w.Write(upstreamIndex(1))
					return
				}
				req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, origin.URL+"/index.yaml", http.NoBody)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				req.Header = r.Header.Clone()
				response, err := origin.Client().Do(req)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer func() { _ = response.Body.Close() }()
				w.Header().Set("ETag", response.Header.Get("ETag"))
				w.WriteHeader(response.StatusCode)
				_, _ = io.Copy(w, response.Body)
			}))
			t.Cleanup(gateway.Close)
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			require.NoError(t, store.SetWithValidators("proxy", upstreamIndex(1), `"old"`, ""))
			client := NewClient("http://charts.example.invalid", HelmCreds{}, false, gateway.URL, "").(*nativeHelmChart)
			var body []byte
			var err error
			switch policy {
			case "expired":
				regressionExpire(t, store, "proxy")
				body, err = store.Revalidate(t.Context(), "proxy", client)
			case "always fresh":
				client.indexCache = retainedIndexView{&HelmIndexRecord{Body: upstreamIndex(1), ETag: `"old"`}}
				body, err = client.loadRepoIndex(t.Context(), 1<<20)
			case "forced":
				body, err = store.ForceRefresh(t.Context(), "proxy", client)
			}
			require.NoError(t, err)
			require.Equal(t, upstreamIndex(2), body, "freshness checks must reach the origin through a caching proxy")
			require.EqualValues(t, 1, originRequests.Load())
		})
	}
}
