package helm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexStoreDisabled(t *testing.T) {
	s := NewIndexStore("", time.Hour, 1<<30)
	_, err := s.Get("https://example.com/index.yaml")
	require.Error(t, err)
	err = s.Set("https://example.com/index.yaml", []byte(regressionIndexBody))
	require.Error(t, err)
	err = s.CopyTo("https://example.com/index.yaml", []byte(regressionIndexBody), t.TempDir(), "repo")
	require.Error(t, err)
}

func TestIndexStoreSetGet(t *testing.T) {
	dir := t.TempDir()
	s := NewIndexStore(dir, time.Hour, 1<<30)
	url := "https://example.com/index.yaml"
	data := []byte("apiVersion: v1\nentries: {}\n")

	require.NoError(t, s.Set(url, data))

	got, err := s.Get(url)
	require.NoError(t, err)
	require.Equal(t, data, got)

	// A different URL must be a miss.
	_, err = s.Get("https://other.example.com/index.yaml")
	require.Error(t, err)
}

func TestIndexStoreEmptyDataRejected(t *testing.T) {
	s := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
	require.Error(t, s.Set("https://example.com/index.yaml", nil))
}

func TestIndexStoreExpiration(t *testing.T) {
	dir := t.TempDir()
	s := NewIndexStore(dir, time.Hour, 1<<30)
	url := "https://example.com/index.yaml"

	require.NoError(t, s.Set(url, []byte(regressionIndexBody)))
	_, err := s.Get(url)
	require.NoError(t, err)

	// Backdate the file beyond the expiration.
	file := s.cacheFileName(url)
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(file, old, old))

	_, err = s.Get(url)
	require.Error(t, err)
}

func TestIndexStoreOverwrite(t *testing.T) {
	s := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
	url := "https://example.com/index.yaml"

	require.NoError(t, s.Set(url, []byte("apiVersion: v1\nentries: {}\n#old\n")))
	require.NoError(t, s.Set(url, []byte("apiVersion: v1\nentries: {}\n#new\n")))
	got, err := s.Get(url)
	require.NoError(t, err)
	require.Equal(t, []byte("apiVersion: v1\nentries: {}\n#new\n"), got)
}

func TestIndexStoreCopyTo(t *testing.T) {
	s := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
	url := "https://example.com/index.yaml"
	data := []byte("apiVersion: v1\nentries: {}\n")
	destDir := t.TempDir()

	require.NoError(t, s.Set(url, data))
	require.NoError(t, s.CopyTo(url, data, destDir, "myrepo"))

	copied, err := os.ReadFile(filepath.Join(destDir, "myrepo-index.yaml"))
	require.NoError(t, err)
	require.Equal(t, data, copied)
}

func TestCacheIndexFileName(t *testing.T) {
	require.Equal(t, "myrepo-index.yaml", cacheIndexFileName("myrepo"))
	require.Equal(t, "index.yaml", cacheIndexFileName(""))
}

func TestIndexStoreConcurrentSetGet(t *testing.T) {
	s := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
	url := "https://example.com/index.yaml"
	done := make(chan struct{}, 8)

	for range 8 {
		go func() {
			for range 25 {
				_ = s.Set(url, []byte(regressionIndexBody))
				_, _ = s.Get(url)
			}
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
}

// TestDependencyBuildSkipsRefreshWhenStoreAttached verifies the command line
// helm is invoked with changes only when a persistent index store is attached.
// The behavior is exercised end-to-end in reposerver e2e; the flag gating
// logic is a single comparison covered by the build-time wiring tests.
func TestDependencyBuildSkipsRefreshWhenStoreAttached(t *testing.T) {
	t.Skip("covered by TestDependencyBuildSkipRefreshOnlyWhenAllWarm; kept for name parity with old suite")
}

// Pause the sweep just before it takes the publication lock. A refresh can
// then replace the old candidate, deterministically exercising the interval
// between discovering an expired file and deciding whether to remove it.
func TestIndexStoreSweepRechecksRefreshedCandidate(t *testing.T) {
	t.Run("same store", func(t *testing.T) { testSweepRefresh(t, false) })
	t.Run("another store instance", func(t *testing.T) { testSweepRefresh(t, true) })
}

func testSweepRefresh(t *testing.T, anotherStore bool) {
	t.Helper()
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	publisher := store
	if anotherStore {
		publisher = NewIndexStore(store.dir, time.Hour, 1<<20)
	}
	identity := "refresh-during-sweep"
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	old := time.Now().Add(-48 * time.Hour)
	file := store.cacheFileName(identity)
	require.NoError(t, os.Chtimes(file, old, old))
	entered, resumeSweep := make(chan struct{}), make(chan struct{})
	var paused atomic.Bool
	acquire := store.lock
	store.lock = func(ctx context.Context, key string, shared, wait bool) (func(), error) {
		if key == file && !wait && paused.CompareAndSwap(false, true) {
			close(entered)
			<-resumeSweep
		}
		return acquire(ctx, key, shared, wait)
	}
	var release sync.Once
	resume := func() { release.Do(func() { close(resumeSweep) }) }
	t.Cleanup(resume)
	done := make(chan struct{})
	go func() {
		store.Sweep()
		close(done)
	}()
	select {
	case <-entered:
	case <-done:
		t.Fatal("sweep removed its candidate without coordinating with the publication lock")
	case <-time.After(3 * time.Second):
		t.Fatal("sweep did not reach the publication lock")
	}
	fresh := []byte(regressionIndexBody + "# refreshed\n")
	require.NoError(t, publisher.Set(identity, fresh))
	resume()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sweep did not finish after the refresh")
	}
	got, err := store.Get(identity)
	require.NoError(t, err, "the replacement is fresh and must survive eviction")
	require.Equal(t, fresh, got)
}

// Run the store in actual independent processes, rather than giving goroutines
// separate store instances backed by one process's mutex map.
func TestIndexStoreProcessHelper(t *testing.T) {
	action := os.Getenv("ARGOCD_INDEX_TEST_ACTION")
	if action == "" {
		return
	}
	store := NewIndexStore(os.Getenv("ARGOCD_INDEX_TEST_DIR"), time.Hour, 1<<20)
	if action == "hold" {
		unlock, err := store.lock(t.Context(), store.cacheFileName("shared"), false, true)
		require.NoError(t, err)
		defer unlock()
		// Model a process dying between writing a temporary file and rename.
		require.NoError(t, os.WriteFile(store.cacheFileName("shared")+".tmp-orphan", []byte("partial"), 0o600))
		fmt.Println("ready")
		_, err = io.Copy(io.Discard, os.Stdin)
		require.NoError(t, err)
		return
	}
	if action == "fetch-wait" {
		acquire := store.lock
		store.lock = func(ctx context.Context, file string, shared, wait bool) (func(), error) {
			if wait && !shared {
				unlock, err := acquire(ctx, file, shared, false)
				if err == nil {
					unlock()
					t.Fatal("second fetch must contend with the first process")
				}
				require.ErrorIs(t, err, errIndexLockBusy)
				fmt.Println("contended")
			}
			return acquire(ctx, file, shared, wait)
		}
	} else {
		fmt.Println("ready")
	}
	client := NewClient(os.Getenv("ARGOCD_INDEX_TEST_URL"), HelmCreds{}, false, "", "").(*nativeHelmChart)
	body, err := store.Revalidate(t.Context(), "shared", client)
	require.NoError(t, err)
	require.Equal(t, regressionIndexBody, string(body))
}

type indexTestProcess struct {
	*exec.Cmd
	finished chan struct{}
	err      error
}

func (p *indexTestProcess) Wait() error {
	<-p.finished
	return p.err
}

func startIndexProcess(t *testing.T, action, dir, url string) *indexTestProcess {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestIndexStoreProcessHelper$", "-test.timeout=10s")
	command.Env = append(os.Environ(), "ARGOCD_INDEX_TEST_ACTION="+action, "ARGOCD_INDEX_TEST_DIR="+dir, "ARGOCD_INDEX_TEST_URL="+url)
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	stdin, err := command.StdinPipe()
	require.NoError(t, err)
	command.Stderr = os.Stderr
	require.NoError(t, command.Start())
	process := &indexTestProcess{Cmd: command, finished: make(chan struct{})}
	t.Cleanup(func() {
		_ = stdin.Close()
		select {
		case <-process.finished:
		default:
			_ = command.Process.Kill()
		}
		_ = process.Wait()
	})
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		expected := "ready"
		if action == "fetch-wait" {
			expected = "contended"
		}
		ready <- scanner.Scan() && scanner.Text() == expected
		_, _ = io.Copy(io.Discard, stdout)
		process.err = command.Wait()
		close(process.finished)
	}()
	select {
	case ok := <-ready:
		require.True(t, ok, "helper exited before reaching its barrier")
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not reach its barrier")
	}
	return process
}

func TestIndexStoreProcessLockCancellationAndCrashRecovery(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, regressionIndexBody)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	holder := startIndexProcess(t, "hold", dir, server.URL)
	// Startup and cleanup must skip active records rather than hang on a lock.
	store := NewIndexStore(dir, time.Hour, 1<<20)
	client := NewClient(server.URL, HelmCreds{}, false, "", "").(*nativeHelmChart)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := store.Revalidate(ctx, "shared", client)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, requests.Load(), "a cancelled waiter must never fetch")
	require.NoError(t, holder.Process.Kill())
	require.Error(t, holder.Wait())
	store.Sweep()
	_, err = os.Stat(store.cacheFileName("shared") + ".tmp-orphan")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = store.Revalidate(t.Context(), "shared", client)
	require.NoError(t, err, "the kernel must release a crashed process's lock")
	require.EqualValues(t, 1, requests.Load())
	info, err := os.Stat(store.cacheFileName("shared") + ".lock")
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	store.Sweep()
	after, err := os.Stat(store.cacheFileName("shared") + ".lock")
	require.NoError(t, err)
	require.True(t, os.SameFile(info, after), "cleanup must never unlink lock files")
}

func TestIndexStoreProcessesShareColdFetch(t *testing.T) {
	var requests atomic.Int32
	entered, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, regressionIndexBody)
	}))
	t.Cleanup(server.Close)
	dir := filepath.Join(t.TempDir(), "new-cache")
	first := startIndexProcess(t, "fetch", dir, server.URL)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first process did not fetch")
	}
	second := startIndexProcess(t, "fetch-wait", dir, server.URL)
	close(release)
	require.NoError(t, first.Wait())
	require.NoError(t, second.Wait())
	require.EqualValues(t, 1, requests.Load(), "independent processes must re-read the warm record after locking")
}

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
		store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
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
		regressionExpire(t, store, identity)
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
		store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
		identity := "https://example.com\x00fp"

		_, err := store.Revalidate(context.Background(), identity, newChartClient(srv.URL))
		require.NoError(t, err)
		regressionExpire(t, store, identity)
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
	etag, _ := sharedInMemoryValidators.getForBody(srv.URL, cache.data)
	require.Equal(t, `"old"`, etag)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	etag, modified := sharedInMemoryValidators.getForBody(srv.URL, cache.data)
	assert.Empty(t, etag, "a new 200 replaces validator metadata, including removing it")
	assert.Empty(t, modified)
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
	_, err := os.Stat(store.cacheFileName(orphan))
	require.ErrorIs(t, err, os.ErrNotExist, "the orphan must actually be removed from disk")
	_, err = store.GetRetained(orphan)
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

func TestIndexStoreSweepPreservesUnownedFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewIndexStore(dir, time.Minute, 1<<20)
	names := []string{"unrelated.json", "backup.json.tmp-data", strings.Repeat("a", 63) + ".json", strings.Repeat("A", 64) + ".json", strings.Repeat("g", 64) + ".json", strings.Repeat("a", 64) + ".json.tmp-"}
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range names {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("unrelated user data"), 0o600))
		require.NoError(t, os.Chtimes(path, old, old))
	}
	store.Sweep()
	for _, name := range names {
		assert.FileExists(t, filepath.Join(dir, name), "sweep must only delete its own filenames")
	}
}

func TestIndexStoreLegacyValidatorUpdatePreservesDeadline(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.Set("repo", []byte(regressionIndexBody)))
	path := store.cacheFileName("repo")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var record indexRecord
	require.NoError(t, json.Unmarshal(raw, &record))
	record.StoredAt = ""
	raw, err = json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	info, err := os.Stat(path)
	require.NoError(t, err)
	_, err = store.Get("repo")
	require.Error(t, err)
	require.NoError(t, store.UpdateValidators("repo", `"updated"`, ""))
	retained, err := store.readRecord("repo", false)
	require.NoError(t, err)
	require.Equal(t, `"updated"`, retained.ETag)
	require.Equal(t, regressionIndexBody, retained.Body)
	require.Equal(t, info.ModTime().UTC().Format(time.RFC3339Nano), retained.StoredAt)
	_, err = store.Get("repo")
	require.Error(t, err, "validator-only update must leave the legacy record expired")
}
