package helm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests assert the cache's externally relevant guarantees. They deliberately
// fail when a regression exists; a passing happy-path fetch is not sufficient.
const regressionIndexBody = "apiVersion: v1\nentries: {}\n"

func regressionClient(url string) *nativeHelmChart {
	return NewClient(url, HelmCreds{}, false, "", "").(*nativeHelmChart)
}

// A timeout here also catches recursive-lock deadlocks. Context cancellation
// alone cannot bound code stuck in a non-cancellable mutex acquisition.
func regressionRevalidate(t *testing.T, store *IndexStore, identity string, client *nativeHelmChart) ([]byte, error) {
	t.Helper()
	type result struct {
		data []byte
		err  error
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan result, 1)
	go func() {
		data, err := store.Revalidate(ctx, identity, client)
		done <- result{data: data, err: err}
	}()
	select {
	case got := <-done:
		return got.data, got.err
	case <-ctx.Done():
		t.Fatal("index revalidation did not finish within 3 seconds; check locking and cancellation")
		return nil, ctx.Err()
	}
}

func regressionExpire(t *testing.T, store *IndexStore, identity string) {
	t.Helper()
	old := time.Now().Add(-2 * store.expiration)
	require.NoError(t, os.Chtimes(store.cacheFileName(identity), old, old))
}

func TestIndexCacheRegressionWarmBuildPreservesFreshness(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	repo := HelmRepository{Name: "repo", Repo: "https://example.invalid", Creds: HelmCreds{}}
	identity := repoIdentity(repo)
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	old := time.Now().Add(-30 * time.Minute)
	require.NoError(t, os.Chtimes(store.cacheFileName(identity), old, old))
	before, err := os.Stat(store.cacheFileName(identity))
	require.NoError(t, err)

	var commands [][]string
	cmd, err := newCmdWithVersion(t.TempDir(), false, "", "", func(c *exec.Cmd, _ func(string) string) (string, error) {
		commands = append(commands, c.Args)
		return "", nil
	})
	require.NoError(t, err)
	t.Cleanup(cmd.Close)
	cmd.persistentIndexStore = store
	h := &helm{cmd: *cmd, repos: []HelmRepository{repo}}
	require.NoError(t, h.DependencyBuild(t.Context()))
	require.Len(t, commands, 1, "a warm build must not register repositories with helm repo add")
	require.Contains(t, commands[0], "--skip-refresh")
	after, err := os.Stat(store.cacheFileName(identity))
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(), "copying an unchanged snapshot is not origin validation")
}

func TestIndexCacheRegressionFirstFetchKeepsValidators(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"first"`)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(srv.Close)
	// The directory must not already exist: first-use initialization is part of
	// this regression, and t.TempDir() alone would hide it.
	store := NewIndexStore(filepath.Join(t.TempDir(), "new-store"), time.Hour, 1<<20)
	data, err := regressionRevalidate(t, store, "identity", regressionClient(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, regressionIndexBody, string(data))
	etag, _, err := store.GetHelmIndexValidators("identity")
	require.NoError(t, err)
	assert.Equal(t, `"first"`, etag)
}

func TestIndexCacheRegressionExpiredUnchangedBodyUses304(t *testing.T) {
	var requests, notModified atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"first"`)
		if r.Header.Get("If-None-Match") == `"first"` {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(srv.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	_, err := regressionRevalidate(t, store, "identity", regressionClient(srv.URL))
	require.NoError(t, err)
	regressionExpire(t, store, "identity")
	data, err := regressionRevalidate(t, store, "identity", regressionClient(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, regressionIndexBody, string(data))
	assert.Equal(t, int32(2), requests.Load())
	assert.Equal(t, int32(1), notModified.Load(), "expiry must retain the body AND its matching validators")
}

func TestIndexCacheRegressionConcurrentMissesFetchOnce(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(srv.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	const callers = 16
	ready := sync.WaitGroup{}
	ready.Add(callers)
	start := make(chan struct{})
	results := make(chan error, callers)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for range callers {
		go func() {
			ready.Done()
			<-start
			data, err := store.Revalidate(ctx, "one-identity", regressionClient(srv.URL))
			if err == nil && string(data) != regressionIndexBody {
				err = errors.New("concurrent lookup returned unexpected index bytes")
			}
			results <- err
		}()
	}
	ready.Wait()
	close(start)
	for range callers {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("concurrent cache misses did not complete; possible recursive-lock deadlock")
		}
	}
	assert.Equal(t, int32(1), requests.Load(), "one shared identity must have one ordinary fetch")
}

func TestIndexCacheRegressionOriginFailureDoesNotServeExpiredBody(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(srv.Close)
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			require.NoError(t, store.Set("identity", []byte(regressionIndexBody)))
			regressionExpire(t, store, "identity")
			data, err := regressionRevalidate(t, store, "identity", regressionClient(srv.URL))
			require.Error(t, err)
			assert.Empty(t, data)
			_, err = store.Get("identity")
			assert.Error(t, err, "a failed origin validation must not renew freshness")
		})
	}
}

func TestIndexCacheRegression200WithoutValidatorsClearsOldOnes(t *testing.T) {
	cache := &fakeValidatorsCache{}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("ETag", `"old"`)
			w.Header().Set("Last-Modified", "Wed, 21 Oct 2026 07:28:00 GMT")
		}
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, `"old"`, cache.etag)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Empty(t, cache.etag, "a new 200 replaces validator metadata, including removing it")
	assert.Empty(t, cache.lastModified)
}

// Simulate Redis accepting metadata but failing its separate body write. The
// next conditional request must not label the old body with the new body's ETag.
type regressionFailingBodyCache struct {
	fakeValidatorsCache
	failBodyWrite bool
}

func (c *regressionFailingBodyCache) SetHelmIndex(repo string, body []byte) error {
	if c.failBodyWrite {
		c.failBodyWrite = false
		return errors.New("injected cache body-write failure")
	}
	return c.fakeIndexCache.SetHelmIndex(repo, body)
}

func TestIndexCacheRegressionBodyWriteFailureCannotPairWrongETag(t *testing.T) {
	cache := &regressionFailingBodyCache{}
	var generation atomic.Int32
	generation.Store(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version, etag := "1.0.0", `"v1"`
		if generation.Load() == 2 {
			version, etag = "2.0.0", `"v2"`
		}
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte("apiVersion: v1\nentries:\n  child:\n    - version: " + version + "\n"))
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	generation.Store(2)
	cache.failBodyWrite = true
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err, "a cache-write failure must not discard a successful origin response")
	require.Equal(t, "2.0.0", index.Entries["child"][0].Version)
	index, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version, "304 must never return the previous generation")
}

func TestIndexCacheRegressionOversizedResponseNotCached(t *testing.T) {
	const prefix = "apiVersion: v1\nentries: {}\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(prefix + "# body exceeds the configured limit\n"))
	}))
	t.Cleanup(srv.Close)
	cache := &fakeValidatorsCache{}
	client := NewClient(srv.URL, HelmCreds{}, false, "", "", WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, int64(len(prefix)))
	require.ErrorContains(t, err, "exceeds maximum size")
	require.Empty(t, cache.data, "a valid YAML prefix must never publish a truncated index")
}

// Run under -race. Keys differ so a per-repository fetch lock cannot hide a
// race in the one shared fallback validator map.
func TestIndexCacheRegressionConcurrentValidatorMap(t *testing.T) {
	t.Helper() // the -race detector is the actual assertion engine here
	validators := newInMemoryIndexValidators()
	var workers sync.WaitGroup
	for _, repo := range []string{"one", "two", "three", "four"} {
		workers.Go(func() {
			for range 100 {
				_ = validators.SetHelmIndexValidators(repo, `"etag"`, "")
				_, _, _ = validators.GetHelmIndexValidators(repo)
				_ = validators.SetHelmIndexValidators(repo, "", "")
			}
		})
	}
	workers.Wait()
}

// Finding 1 (round 2): the retention prune inside readRecord must not take
// the identity lock again while Revalidate holds it. A store entry past
// retention (long idle) + a revalidation must complete, not hang.
func TestIndexCacheRegressionRetentionPruneDoesNotDeadlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(srv.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	identity := "prune-identity"
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	// Push the file's mtime past freshness AND retention.
	regressionExpire(t, store, identity)
	old := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(store.cacheFileName(identity), old, old))
	// Past retention: readRecord prunes. Under the previous implementation
	// this re-acquired the lock Revalidate holds and hung.
	_, err := regressionRevalidate(t, store, identity, regressionClient(srv.URL))
	require.NoError(t, err)
	// The pruned entry must be gone; revalidation republished a fresh record.
	rec, err := store.readRecord(identity, true)
	require.NoError(t, err)
	assert.NotEmpty(t, rec.Body)
}

// Finding 5 (round 2): a 200 whose body is an HTML error page (origin
// recovered-with-garbage, captive portal, proxy) must fail validation and be
// cached as fresh by neither Revalidate nor ForceRefresh.
func TestIndexCacheRegressionRevalidateRejectsMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	}))
	t.Cleanup(srv.Close)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	identity := "html-identity"
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	regressionExpire(t, store, identity)
	_, err := regressionRevalidate(t, store, identity, regressionClient(srv.URL))
	require.Error(t, err, "an HTML error page must not be cached as a fresh index")
	rec, rerr := store.readRecord(identity, false)
	require.NoError(t, rerr, "the previous good entry must remain (retained, not overwritten)")
	assert.Equal(t, regressionIndexBody, rec.Body, "store content must be unchanged after a rejected fetch")
}

// Finding 2 (round 3): entries whose identity is never read again (orphaned
// by credential rotation) must still be evicted once past retention.
func TestIndexCacheRegressionSweepEvictsOrphanedEntries(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	orphan := "orphaned-identity"
	require.NoError(t, store.Set(orphan, []byte(regressionIndexBody)))
	// Push the orphan past freshness AND retention via its publication mtime.
	old := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(store.cacheFileName(orphan), old, old))

	// A sweep (as run at init or after another repo's publication) must
	// remove the past-retention orphan even though nothing reads its identity.
	store.Sweep()
	_, err := store.GetRetained(orphan)
	require.Error(t, err, "an orphaned entry past retention must be evicted by the sweep")

	// A fresh entry is never swept.
	kept := "fresh-identity"
	require.NoError(t, store.Set(kept, []byte(regressionIndexBody)))
	store.Sweep()
	_, err = store.Get(kept)
	require.NoError(t, err, "fresh entries must survive the sweep")
}

// rotatingCreds simulates a dynamic credential provider (Azure workload
// identity): every GetPassword call returns the next token.
type rotatingCreds struct {
	mu     sync.Mutex
	tokens []string
	calls  int
}

func (r *rotatingCreds) GetUsername() string { return "reader" }
func (r *rotatingCreds) GetPassword() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls >= len(r.tokens) {
		r.calls = len(r.tokens) - 1 // last token persists (cached)
	}
	token := r.tokens[r.calls]
	r.calls++
	return token, nil
}
func (r *rotatingCreds) GetCAPath() string           { return "" }
func (r *rotatingCreds) GetCertData() []byte         { return nil }
func (r *rotatingCreds) GetKeyData() []byte          { return nil }
func (r *rotatingCreds) GetInsecureSkipVerify() bool { return false }

// Finding 1 (round 3): a rotating provider must have its credentials
// resolved ONCE per operation so the HTTP request and the cache-scope
// fingerprint describe the SAME token. Two constructions that would each
// call GetPassword (one for auth, one for the key) must not diverge.
func TestIndexCacheRegressionRotatingCredsShareOneTokenView(t *testing.T) {
	creds := &rotatingCreds{tokens: []string{"token-A", "token-B"}}

	snapA, err := snapshotCreds(creds)
	require.NoError(t, err)
	scopeA := credsScope(snapA)

	snapB, err := snapshotCreds(creds)
	require.NoError(t, err)
	scopeB := credsScope(snapB)

	// Each resolved snapshot is internally consistent: the scope derives
	// from the same password the HTTP layer will send.
	pwA, _ := snapA.GetPassword()
	assert.Equal(t, "token-A", pwA, "snapshot A pins token A")
	pwB, _ := snapB.GetPassword()
	assert.Equal(t, "token-B", pwB, "snapshot B pins token B")
	assert.NotEqual(t, scopeA, scopeB, "different resolved tokens must map to different cache scopes")

	// And a snapshot's scope is stable regardless of further rotations —
	// the cache identity cannot drift away from the token in flight.
	assert.Equal(t, scopeA, credsScope(snapA), "snapshot A's scope must not change after later rotations")
}
