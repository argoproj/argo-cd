package helm

// Pinned-Helm integration tests for the disk-only dependency index reuse
// slice (PR2 of the #30997 rebuild). Everything is local: a packaged
// dependency, a versioned index served by httptest, and request accounting.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type regressionIndexRequest struct {
	status int
	bytes  int
	etag   string
}

type regressionChartRepo struct {
	server   *httptest.Server
	mu       sync.Mutex
	version  string
	password string
	requests []regressionIndexRequest
	archives int
	packages map[string][]byte
}

func regressionRequirePinnedHelm(tb testing.TB) {
	tb.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		tb.Skip("integration test requires repository-pinned Helm in PATH")
	}
	versions, err := os.ReadFile(filepath.Join("..", "..", "hack", "tool-versions.sh"))
	require.NoError(tb, err)
	var pinned string
	for line := range strings.SplitSeq(string(versions), "\n") {
		if value, ok := strings.CutPrefix(line, "HELM_VERSION="); ok {
			pinned = strings.Trim(value, "\"'")
		}
	}
	require.NotEmpty(tb, pinned)
	version, err := exec.CommandContext(tb.Context(), "helm", "version", "--short").Output()
	require.NoError(tb, err)
	short, _, _ := strings.Cut(strings.TrimSpace(string(version)), "+")
	require.Equal(tb, "v"+pinned, short, "put the pinned binary first in PATH; do not silently validate a different Helm")
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "HELM_REPOSITORY_CONFIG", "HELM_REPOSITORY_CACHE", "HELM_REGISTRY_CONFIG"} {
		tb.Setenv(name, "")
		require.NoError(tb, os.Unsetenv(name))
	}
}

func regressionPackage(tb testing.TB, name, version string) []byte {
	tb.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gzipWriter)
	files := []struct{ name, body string }{
		{name + "/Chart.yaml", "apiVersion: v2\nname: " + name + "\nversion: " + version + "\n"},
		{name + "/templates/configmap.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cache-regression\ndata:\n  version: {{ .Chart.Version | quote }}\n"},
	}
	for _, file := range files {
		require.NoError(tb, archive.WriteHeader(&tar.Header{Name: file.name, Mode: 0o600, Size: int64(len(file.body))}))
		_, err := archive.Write([]byte(file.body))
		require.NoError(tb, err)
	}
	require.NoError(tb, archive.Close())
	require.NoError(tb, gzipWriter.Close())
	return buffer.Bytes()
}

func newRegressionChartRepo(t *testing.T) *regressionChartRepo {
	t.Helper()
	f := &regressionChartRepo{
		version:  "1.0.0",
		packages: map[string][]byte{"1.0.0": regressionPackage(t, "child", "1.0.0"), "2.0.0": regressionPackage(t, "child", "2.0.0")},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	return f
}

func (f *regressionChartRepo) index() []byte {
	versions := []string{f.version}
	if f.version != "1.0.0" {
		versions = append(versions, "1.0.0")
	}
	var builder strings.Builder
	builder.WriteString("apiVersion: v1\nentries:\n  child:\n")
	for _, version := range versions {
		fmt.Fprintf(&builder, "    - apiVersion: v2\n      name: child\n      version: %s\n      urls: [child-%s.tgz]\n", version, version)
	}
	return []byte(builder.String())
}

func (f *regressionChartRepo) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := r.URL.Path == "/index.yaml"
	if index {
		etag := `"` + f.version + `"`
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			f.requests = append(f.requests, regressionIndexRequest{status: http.StatusNotModified, etag: etag})
			w.WriteHeader(http.StatusNotModified)
			return
		}
		body := f.index()
		f.requests = append(f.requests, regressionIndexRequest{status: http.StatusOK, bytes: len(body), etag: r.Header.Get("If-None-Match")})
		_, _ = w.Write(body)
		return
	}
	for version, data := range f.packages {
		if r.URL.Path == "/child-"+version+".tgz" {
			f.archives++
			_, _ = w.Write(data)
			return
		}
	}
	http.NotFound(w, r)
}

func (f *regressionChartRepo) snapshot() ([]regressionIndexRequest, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]regressionIndexRequest(nil), f.requests...), f.archives
}

func (f *regressionChartRepo) publish(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.version = version
}

func regressionParent(t *testing.T, repository string) string {
	t.Helper()
	dir := t.TempDir()
	chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: '>=1.0.0 <3.0.0'\n    repository: %q\n", repository)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
	return dir
}

func regressionBuild(t *testing.T, dir string, repo HelmRepository, store *IndexStore, force bool) (Helm, error) {
	t.Helper()
	h, err := NewHelmAppWithIndexStore(dir, []HelmRepository{repo}, false, "", "", "", false, false, store, force)
	require.NoError(t, err)
	t.Cleanup(h.Dispose)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.DependencyBuild(ctx) }()
	select {
	case err := <-done:
		return h, err
	case <-ctx.Done():
		t.Fatal("real Helm dependency build did not finish within 30 seconds; check cache deadlock")
		return h, ctx.Err()
	}
}

func regressionRender(t *testing.T, h Helm, version string) string {
	t.Helper()
	output, _, err := h.Template(&TemplateOpts{Name: "regression", Namespace: "default"})
	require.NoError(t, err)
	require.Contains(t, output, "version: \""+version+"\"")
	return output
}

func regressionExpire(t *testing.T, store *IndexStore, identity string) {
	t.Helper()
	old := time.Now().Add(-2 * store.expiration)
	require.NoError(t, os.Chtimes(store.cacheFileName(identity), old, old))
}

func regressionIdentity(t *testing.T, repo HelmRepository) string {
	t.Helper()
	identity, err := scopedIndexIdentityContext(t.Context(), repo.Repo, credsScope(repo.Creds), "", "")
	require.NoError(t, err)
	return identity
}

// A cold build downloads the index once; a warm build in a fresh chart
// directory reuses the cached index without any HTTP request.
func TestIndexCacheIntegrationColdThenWarm(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(filepath.Join(t.TempDir(), "store"), time.Hour, 1<<20)

	cold, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, cold, "1.0.0")
	requests, archives := fixture.snapshot()
	require.Len(t, requests, 1, "a cold dependency build should fetch each index only once")
	assert.Equal(t, 1, archives)

	warm, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, warm, "1.0.0")
	requests, archives = fixture.snapshot()
	assert.Len(t, requests, 1, "another clean chart directory must reuse the index without GETs")
	assert.Equal(t, 2, archives, "each phase must really download/build its dependency")
}

// An expired record is revalidated with a conditional request; a matching
// ETag yields a 304 and the cached body is reused.
func TestIndexCacheIntegrationExpiredUses304(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	identity := regressionIdentity(t, repo)
	require.NoError(t, store.SetWithValidators(repo.Repo, fixture.index(), `"1.0.0"`, ""))
	record := &HelmIndexRecord{Body: fixture.index(), ETag: `"1.0.0"`, URLIdentity: IndexURLIdentity(fixture.server.URL + "/index.yaml"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	record.BodyDigest = fmt.Sprintf("%x", sha256.Sum256(record.Body))
	require.True(t, store.installRecord(identity, record))
	regressionExpire(t, store, identity)

	h, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h, "1.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1)
	assert.Equal(t, http.StatusNotModified, requests[0].status)
	assert.Zero(t, requests[0].bytes, "304 must save body bytes, not hide a full fetch")
}

// A changed index is refetched through the conditional request (200).
func TestIndexCacheIntegrationExpiredUsesUpdatedIndex(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)

	first, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, first, "1.0.0")

	fixture.publish("2.0.0")
	regressionExpire(t, store, regressionIdentity(t, repo))

	next, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, next, "2.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 2, "ordinary expiry must fetch the changed index exactly once")
	require.Equal(t, http.StatusOK, requests[1].status)
	require.Equal(t, `"1.0.0"`, requests[1].etag, "the expired generation must be conditionally checked")
	require.Positive(t, requests[1].bytes)
}

// Force refresh bypasses freshness and lands the newest index in the store.
func TestIndexCacheIntegrationHardRefresh(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)

	warm, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, warm, "1.0.0")

	fixture.publish("2.0.0")
	hard, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, store, true)
	require.NoError(t, err)
	regressionRender(t, hard, "2.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 2)
	assert.Equal(t, http.StatusOK, requests[1].status)
	assert.Empty(t, requests[1].etag, "a hard refresh must be an unconditional GET")
}

// Disabled store preserves the legacy behavior: every build hits the origin.
func TestIndexCacheIntegrationDisabledPreservesLegacyDownloads(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}

	first, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, nil, false)
	require.NoError(t, err)
	regressionRender(t, first, "1.0.0")
	second, err := regressionBuild(t, regressionParent(t, repo.Repo), repo, nil, false)
	require.NoError(t, err)
	regressionRender(t, second, "1.0.0")
	requests, archives := fixture.snapshot()
	assert.Len(t, requests, 4, "disabled behavior still downloads at registration and refresh for each build")
	assert.Equal(t, 2, archives)
}
