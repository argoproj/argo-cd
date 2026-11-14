package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/pem"
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

// Everything is local: a packaged dependency, a versioned index, and request
// accounting. Tests use fresh parent charts so existing charts/ and Chart.lock
// cannot make a broken cache appear to work.
type regressionChartRepo struct {
	server   *httptest.Server
	mu       sync.Mutex
	version  string
	password string
	requests []regressionIndexRequest
	archives int
	packages map[string][]byte
}

func regressionRequirePinnedHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("integration test requires repository-pinned Helm in PATH")
	}
	versions, err := os.ReadFile(filepath.Join("..", "..", "hack", "tool-versions.sh"))
	require.NoError(t, err)
	var pinned string
	for line := range strings.SplitSeq(string(versions), "\n") {
		if value, ok := strings.CutPrefix(line, "HELM_VERSION="); ok {
			pinned = strings.Trim(value, "\"'")
		}
	}
	require.NotEmpty(t, pinned)
	version, err := exec.CommandContext(t.Context(), "helm", "version", "--short").Output()
	require.NoError(t, err)
	short, _, _ := strings.Cut(strings.TrimSpace(string(version)), "+")
	actual := short
	require.Equal(t, "v"+pinned, actual, "put the pinned binary first in PATH; do not silently validate a different Helm")
	// Remove overrides rather than setting them empty: Helm can interpret an
	// explicitly empty repository-cache variable as a relative directory.
	// t.Setenv registers restoration of the original value before Unsetenv.
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "HELM_REPOSITORY_CONFIG", "HELM_REPOSITORY_CACHE", "HELM_REGISTRY_CONFIG"} {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func regressionPackage(t *testing.T, version string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gzipWriter)
	files := []struct{ name, body string }{
		{"child/Chart.yaml", "apiVersion: v2\nname: child\nversion: " + version + "\n"},
		{"child/templates/configmap.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cache-regression\ndata:\n  version: {{ .Chart.Version | quote }}\n"},
	}
	for _, file := range files {
		require.NoError(t, archive.WriteHeader(&tar.Header{Name: file.name, Mode: 0o600, Size: int64(len(file.body))}))
		_, err := archive.Write([]byte(file.body))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NoError(t, gzipWriter.Close())
	return buffer.Bytes()
}

func newRegressionChartRepo(t *testing.T, tls bool, password string) *regressionChartRepo {
	t.Helper()
	f := &regressionChartRepo{
		version: "1.0.0", password: password,
		packages: map[string][]byte{"1.0.0": regressionPackage(t, "1.0.0"), "2.0.0": regressionPackage(t, "2.0.0")},
	}
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(f.serveHTTP))
	if tls {
		f.server.StartTLS()
	} else {
		f.server.Start()
	}
	t.Cleanup(f.server.Close)
	return f
}

func (f *regressionChartRepo) index() []byte {
	// URLs are deliberately relative: the cache must preserve Helm's normal
	// repository-relative archive resolution.
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
	if f.password != "" {
		username, password, ok := r.BasicAuth()
		if !ok || username != "reader" || password != f.password {
			if index {
				f.requests = append(f.requests, regressionIndexRequest{status: http.StatusUnauthorized})
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
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

func regressionParent(t *testing.T, repository string, lock []byte) string {
	t.Helper()
	dir := t.TempDir()
	chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: '>=1.0.0 <3.0.0'\n    repository: %q\n", repository)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
	if lock != nil {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.lock"), lock, 0o600))
	}
	return dir
}

func regressionBuild(t *testing.T, dir string, repo HelmRepository, store *IndexStore, force bool) (Helm, error) {
	t.Helper()
	h, err := NewHelmAppWithIndexStore(dir, []HelmRepository{repo}, false, "", "", "127.0.0.1,localhost", false, false, store, force)
	require.NoError(t, err)
	t.Cleanup(h.Dispose)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.DependencyBuild(ctx) }()
	select {
	case err := <-done:
		return h, err
	case <-ctx.Done():
		t.Fatal("real Helm dependency build did not finish within 5 seconds; check cache deadlock/cancellation")
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

func TestIndexCacheIntegrationColdThenWarm(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, reference := range []string{"url", "alias"} {
		t.Run(reference, func(t *testing.T) {
			fixture := newRegressionChartRepo(t, false, "")
			repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
			store := NewIndexStore(filepath.Join(t.TempDir(), "store"), time.Hour, 1<<20)
			dependency := repo.Repo
			if reference == "alias" {
				dependency = "@fixture"
			}
			cold, err := regressionBuild(t, regressionParent(t, dependency, nil), repo, store, false)
			require.NoError(t, err)
			coldOutput := regressionRender(t, cold, "1.0.0")
			requests, archives := fixture.snapshot()
			require.Len(t, requests, 1, "a cold dependency build should fetch each index only once")
			assert.Equal(t, 1, archives)
			warm, err := regressionBuild(t, regressionParent(t, dependency, nil), repo, store, false)
			require.NoError(t, err)
			assert.Equal(t, coldOutput, regressionRender(t, warm, "1.0.0"))
			requests, archives = fixture.snapshot()
			assert.Len(t, requests, 1, "another clean chart directory must reuse the index without GETs")
			assert.Equal(t, 2, archives, "each phase must really download/build its dependency")
		})
	}
}

func TestIndexCacheIntegrationWarmTLSAndBasicAuth(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, true, "fixture-password")
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw}), 0o600))
	repo := HelmRepository{Name: "private", Repo: fixture.server.URL, Creds: HelmCreds{Username: "reader", Password: "fixture-password", CAPath: caPath}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.Set(scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"), fixture.index()))
	h, err := regressionBuild(t, regressionParent(t, "@private", nil), repo, store, false)
	require.NoError(t, err, "the chart download must receive the custom CA and basic credentials")
	regressionRender(t, h, "1.0.0")
	requests, archives := fixture.snapshot()
	assert.Empty(t, requests)
	assert.Equal(t, 1, archives)
}

func TestIndexCacheIntegrationExpiredUses304(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.Set(scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"), fixture.index()))
	require.NoError(t, store.SetHelmIndexValidators(scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"), `"1.0.0"`, ""))
	regressionExpire(t, store, scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"))
	h, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h, "1.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1)
	assert.Equal(t, http.StatusNotModified, requests[0].status)
	assert.Zero(t, requests[0].bytes, "304 must save body bytes, not hide a full fetch")
}

func TestIndexCacheIntegrationExpiredUsesUpdatedIndex(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	first, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, first, "1.0.0")
	fixture.publish("2.0.0")
	regressionExpire(t, store, scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"))
	next, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, next, "2.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 2, "ordinary expiry must fetch the changed index exactly once")
	require.Equal(t, http.StatusOK, requests[1].status)
	require.Equal(t, `"1.0.0"`, requests[1].etag, "the expired generation must be conditionally checked")
	require.Positive(t, requests[1].bytes)
}

func TestIndexCacheIntegrationHardRefreshAndLockfile(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.Set(scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"), fixture.index()))
	firstDir := regressionParent(t, repo.Repo, nil)
	first, err := regressionBuild(t, firstDir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, first, "1.0.0")
	lock, err := os.ReadFile(filepath.Join(firstDir, "Chart.lock"))
	require.NoError(t, err)
	fixture.publish("2.0.0")
	t.Run("unlocked", func(t *testing.T) {
		h, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, store, true)
		require.NoError(t, err)
		regressionRender(t, h, "2.0.0")
		requests, _ := fixture.snapshot()
		require.Len(t, requests, 1, "hard refresh must not download via both repo add and dependency refresh")
		assert.Empty(t, requests[0].etag, "hard refresh must be unconditional")
	})
	t.Run("locked", func(t *testing.T) {
		before, _ := fixture.snapshot()
		h, err := regressionBuild(t, regressionParent(t, repo.Repo, lock), repo, store, true)
		require.NoError(t, err)
		regressionRender(t, h, "1.0.0")
		after, _ := fixture.snapshot()
		require.Len(t, after, len(before)+1, "a forced locked build must fetch once while preserving the locked version")
	})
}

func TestIndexCacheIntegrationDisabledPreservesLegacyDownloads(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	for range 2 {
		h, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, nil, false)
		require.NoError(t, err)
		regressionRender(t, h, "1.0.0")
	}
	requests, archives := fixture.snapshot()
	assert.Len(t, requests, 4, "disabled behavior still downloads at registration and refresh for each build")
	assert.Equal(t, 2, archives)
}

func TestIndexCacheIntegrationWrongCredentialsDoNotReuseWarmIndex(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "correct-password")
	repo := HelmRepository{Name: "private", Repo: fixture.server.URL, Creds: HelmCreds{Username: "reader", Password: "correct-password"}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	require.NoError(t, store.Set(scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost"), fixture.index()))
	repo.Creds = HelmCreds{Username: "reader", Password: "wrong-password"}
	_, err := regressionBuild(t, regressionParent(t, repo.Repo, nil), repo, store, false)
	require.Error(t, err, "another credential view must not borrow the warm entry")
	requests, archives := fixture.snapshot()
	require.NotEmpty(t, requests, "the invalid credential must be checked with origin")
	assert.Equal(t, http.StatusUnauthorized, requests[0].status)
	assert.Zero(t, archives)
}
