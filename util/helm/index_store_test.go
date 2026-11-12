package helm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	s := NewIndexStore(dir, 50*time.Millisecond, 1<<30)
	url := "https://example.com/index.yaml"

	require.NoError(t, s.Set(url, []byte(regressionIndexBody)))
	_, err := s.Get(url)
	require.NoError(t, err)

	// Backdate the file beyond the expiration.
	file := s.cacheFileName(url)
	old := time.Now().Add(-time.Hour)
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
