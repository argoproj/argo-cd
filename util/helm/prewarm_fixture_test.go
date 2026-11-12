package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureRepo serves a minimal helm repository: an index.yaml (counted) and
// one packaged chart (child-0.1.2).
type fixtureRepo struct {
	srv       *httptest.Server
	indexGets atomic.Int32 // number of index.yaml downloads
	dir       string
}

func writeChartPackage(t *testing.T, dir, name, version string) {
	t.Helper()
	require.NoError(t, osMkdirAll(dir, 0o700))
	files := map[string]string{
		filepath.Join(dir, name, "Chart.yaml"):           fmt.Sprintf("apiVersion: v2\nname: %s\nversion: %s\n", name, version),
		filepath.Join(dir, name, "values.yaml"):          "foo: bar\n",
		filepath.Join(dir, name, "templates", "cm.yaml"): "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\ndata:\n  x: y\n",
	}
	for path, content := range files {
		require.NoError(t, osMkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, osWriteFile(path, []byte(content), 0o600))
	}
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for _, rel := range []string{"Chart.yaml", "values.yaml", filepath.Join("templates", "cm.yaml")} {
		data, err := osReadFile(filepath.Join(dir, name, rel))
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: filepath.Join(name, rel), Mode: 0o600, Size: int64(len(data))}))
		_, err = tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	require.NoError(t, osWriteFile(filepath.Join(dir, name+"-"+version+".tgz"), buf.Bytes(), 0o600))
}

func indexForRepo(baseURL string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
entries:
  child:
    - name: child
      version: 0.1.2
      urls:
        - %s/child-0.1.2.tgz
      created: "2026-01-01T00:00:00Z"
      digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
generated: "2026-01-01T00:00:00Z"
`, baseURL))
}

func newFixtureRepo(t *testing.T) *fixtureRepo {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm binary not available in PATH")
	}
	dir := t.TempDir()
	writeChartPackage(t, dir, "child", "0.1.2")
	fr := &fixtureRepo{dir: dir}
	mux := http.NewServeMux()
	mux.HandleFunc("/child-0.1.2.tgz", func(w http.ResponseWriter, _ *http.Request) {
		data, err := osReadFile(filepath.Join(fr.dir, "child-0.1.2.tgz"))
		require.NoError(t, err)
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
		fr.indexGets.Add(1)
		_, _ = w.Write(indexForRepo(fr.srv.URL))
	})
	fr.srv = httptest.NewServer(mux)
	t.Cleanup(fr.srv.Close)
	return fr
}

// TestDependencyBuildZeroWarmDownloads is the automated stand-in for the
// former env-gated e2e: it proves warm dependency builds run --skip-refresh
// and download ZERO index.yaml files, cold builds download, and a forced
// refresh ignores the store — the exact properties the feature advertises.
func TestDependencyBuildZeroWarmDownloads(t *testing.T) {
	repo := newFixtureRepo(t)
	store := NewIndexStore(t.TempDir(), 1*time.Hour, 1<<30)

	parentWork := t.TempDir() // fresh parent chart dir per phase
	require.NoError(t, osMkdirAll(parentWork, 0o700))
	chartYAML := fmt.Sprintf(`apiVersion: v2
name: parent
version: 0.1.0
dependencies:
  - name: child
    version: ">=0.1.0"
    repository: %s
`, repo.srv.URL)
	require.NoError(t, osWriteFile(filepath.Join(parentWork, "Chart.yaml"), []byte(chartYAML), 0o600))

	repos := []HelmRepository{{Name: "fixture", Repo: repo.srv.URL, Creds: HelmCreds{}}}

	// COLD build: no store entry → helm downloads the index (1 GET), build
	// succeeds, harvest persists the index. workDir is the chart directory.
	h1, err := NewHelmAppWithIndexStore(parentWork, repos, false, "", "", "", false, false, store, false)
	require.NoError(t, err)
	require.NoError(t, h1.DependencyBuild(context.Background()))
	assert.Equal(t, int32(1), repo.indexGets.Load(), "cold build downloads the index once")
	_, err = store.Get(repoIdentity(repos[0]))
	require.NoError(t, err, "harvest must persist the downloaded index")

	// WARM build in a COMPLETELY FRESH work dir: --skip-refresh + prewarmed
	// index → zero index downloads.
	h2, err := NewHelmAppWithIndexStore(parentWork, repos, false, "", "", "", false, false, store, false)
	require.NoError(t, err)
	require.NoError(t, h2.DependencyBuild(context.Background()))
	assert.Equal(t, int32(1), repo.indexGets.Load(), "warm build must not download index.yaml (helm dependency build --skip-refresh)")

	// Forced refresh: policy flag makes the build ignore the store and
	// contact the origin again.
	h3, err := NewHelmAppWithIndexStore(parentWork, repos, false, "", "", "", false, false, store, true)
	require.NoError(t, err)
	require.NoError(t, h3.DependencyBuild(context.Background()))
	assert.Equal(t, int32(2), repo.indexGets.Load(), "forced refresh must contact the origin unconditionally")
}

func osMkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func osWriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
