package helm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	fmt.Println("ready")
	client := NewClient(os.Getenv("ARGOCD_INDEX_TEST_URL"), HelmCreds{}, false, "", "").(*nativeHelmChart)
	body, err := store.Revalidate(t.Context(), "shared", client)
	require.NoError(t, err)
	require.Equal(t, regressionIndexBody, string(body))
}

func startIndexProcess(t *testing.T, action, dir, url string) *exec.Cmd {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestIndexStoreProcessHelper$", "-test.timeout=10s")
	command.Env = append(os.Environ(), "ARGOCD_INDEX_TEST_ACTION="+action, "ARGOCD_INDEX_TEST_DIR="+dir, "ARGOCD_INDEX_TEST_URL="+url)
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	stdin, err := command.StdinPipe()
	require.NoError(t, err)
	command.Stderr = os.Stderr
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = command.Process.Kill()
	})
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && scanner.Text() == "ready"
		_, _ = io.Copy(io.Discard, stdout)
	}()
	select {
	case ok := <-ready:
		require.True(t, ok, "helper exited before reaching its barrier")
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not reach its barrier")
	}
	return command
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
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
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
	second := startIndexProcess(t, "fetch", dir, server.URL)
	close(release)
	require.NoError(t, first.Wait())
	require.NoError(t, second.Wait())
	require.EqualValues(t, 1, requests.Load(), "independent processes must re-read the warm record after locking")
}
