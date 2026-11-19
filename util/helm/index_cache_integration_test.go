package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
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
	server       *httptest.Server
	mu           sync.Mutex
	version      string
	password     string
	requests     []regressionIndexRequest
	archives     int
	packages     map[string][]byte
	extraEntries string
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
	actual := short
	require.Equal(tb, "v"+pinned, actual, "put the pinned binary first in PATH; do not silently validate a different Helm")
	// Remove overrides rather than setting them empty: Helm can interpret an
	// explicitly empty repository-cache variable as a relative directory.
	// tb.Setenv registers restoration of the original value before Unsetenv.
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "HELM_REPOSITORY_CONFIG", "HELM_REPOSITORY_CACHE", "HELM_REGISTRY_CONFIG"} {
		tb.Setenv(name, "")
		require.NoError(tb, os.Unsetenv(name))
	}
}

func regressionPackage(tb testing.TB, version string) []byte {
	tb.Helper()
	return regressionNamedPackage(tb, "child", version)
}

func regressionNamedPackage(tb testing.TB, name, version string) []byte {
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
	builder.WriteString(f.extraEntries)
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
		cancel()
		select {
		case shutdownErr := <-done:
			t.Logf("dependency build stopped after cancellation: %v", shutdownErr)
		case <-time.After(5 * time.Second):
			t.Fatal("real Helm dependency build did not stop within 5 seconds after cancellation")
		}
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
	identity := scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost")
	require.True(t, store.installRecord(identity, &HelmIndexRecord{Body: fixture.index(), ETag: `"1.0.0"`, URLIdentity: IndexURLIdentity(fixture.server.URL + "/index.yaml"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}))
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

func TestIndexCacheIntegrationHTTPAndOCI(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, mode := range []string{"OCI only", "HTTP and OCI", "HTTP and inherited OCI"} {
		t.Run(mode, func(t *testing.T) {
			mixed := mode != "OCI only"
			inherited := mode == "HTTP and inherited OCI"
			packageDir := t.TempDir()
			writeChartPackage(t, packageDir, "oci-child", "1.0.0")
			chart, err := os.ReadFile(filepath.Join(packageDir, "oci-child-1.0.0.tgz"))
			require.NoError(t, err)
			config := []byte(`{"name":"oci-child","version":"1.0.0","apiVersion":"v2"}`)
			configDescriptor := content.NewDescriptorFromBytes("application/vnd.cncf.helm.config.v1+json", config)
			layer := content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chart)
			manifest, err := json.Marshal(imagev1.Manifest{SchemaVersion: 2, MediaType: imagev1.MediaTypeImageManifest, Config: configDescriptor, Layers: []imagev1.Descriptor{layer}})
			require.NoError(t, err)
			manifestDescriptor := content.NewDescriptorFromBytes(imagev1.MediaTypeImageManifest, manifest)
			blobs := map[string][]byte{configDescriptor.Digest.String(): config, layer.Digest.String(): chart}
			var manifestRequests atomic.Int32
			registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
				username, password, ok := r.BasicAuth()
				if !ok || username != "registry-reader" || password != "registry-secret" {
					w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.URL.Path == "/v2/" {
					return
				}
				if reference, ok := strings.CutPrefix(r.URL.Path, "/v2/oci-child/manifests/"); ok {
					if reference != "1.0.0" && reference != manifestDescriptor.Digest.String() {
						http.NotFound(w, r)
						return
					}
					manifestRequests.Add(1)
					w.Header().Set("Content-Type", imagev1.MediaTypeImageManifest)
					w.Header().Set("Docker-Content-Digest", manifestDescriptor.Digest.String())
					w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
					_, _ = w.Write(manifest)
					return
				}
				if body, ok := blobs[strings.TrimPrefix(r.URL.Path, "/v2/oci-child/blobs/")]; ok {
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Header().Set("Content-Length", strconv.Itoa(len(body)))
					_, _ = w.Write(body)
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(registry.Close)
			registryURL := strings.TrimPrefix(registry.URL, "http://")
			registryFile := filepath.Join(t.TempDir(), "config.json")
			registryConfig := []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, registryURL, base64.StdEncoding.EncodeToString([]byte("registry-reader:registry-secret"))))
			if inherited {
				require.NoError(t, os.WriteFile(registryFile, registryConfig, 0o600))
				t.Setenv("HELM_REGISTRY_CONFIG", registryFile)
				t.Setenv("DOCKER_CONFIG", t.TempDir())
			}
			fixture := newRegressionChartRepo(t, false, "")
			fixture.mu.Lock()
			fixture.extraEntries = "  unrelated:\n    - apiVersion: v2\n      name: unrelated\n      version: 1.0.0\n      urls: [unused.tgz]\n"
			fixture.mu.Unlock()
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			store.FilterEnabled = true
			for range 2 {
				dir := t.TempDir()
				chart := fmt.Sprintf("apiVersion: v2\nname: mixed\nversion: 1.0.0\ndependencies:\n  - name: oci-child\n    alias: registry\n    version: 1.0.0\n    repository: 'oci://%s'\n", registryURL)
				repos := []HelmRepository{{Name: "registry", Repo: registryURL, Creds: HelmCreds{Username: "registry-reader", Password: "registry-secret"}, EnableOci: true, InsecureOCIForceHttp: true}}
				if inherited {
					repos[0].Creds = HelmCreds{}
				}
				wantCharts := 1
				if mixed {
					chart += "  - name: child\n    alias: web\n    version: 1.0.0\n    repository: '@web'\n"
					repos = append(repos, HelmRepository{Name: "web", Repo: fixture.server.URL, Creds: HelmCreds{}})
					wantCharts++
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
				app, err := NewHelmAppWithIndexStore(dir, repos, false, "", "", "", false, false, store, false)
				require.NoError(t, err)
				t.Cleanup(app.Dispose)
				require.NoError(t, app.DependencyBuild(t.Context()))
				output, _, err := app.Template(&TemplateOpts{Name: "mixed", Namespace: "default"})
				require.NoError(t, err)
				require.Equal(t, wantCharts, strings.Count(output, "kind: ConfigMap"))
			}
			requests, _ := fixture.snapshot()
			if mixed {
				if inherited {
					require.Len(t, requests, 4, "inherited Helm registry credentials retain the complete legacy build path")
					unchanged, err := os.ReadFile(registryFile)
					require.NoError(t, err)
					require.Equal(t, registryConfig, unchanged)
				} else {
					require.Len(t, requests, 1, "the warm mixed build must reuse only the HTTP index")
				}
			} else {
				require.Empty(t, requests, "OCI-only builds must not fetch an HTTP index")
				files, err := filepath.Glob(filepath.Join(store.dir, "*.json"))
				require.NoError(t, err)
				require.Empty(t, files, "OCI content must never be published as an index record")
			}
			require.GreaterOrEqual(t, manifestRequests.Load(), int32(2), "OCI resolution and registry authentication must still execute")
		})
	}
}

func TestIndexCacheConfiguredNameCannotShadowGeneratedName(t *testing.T) {
	regressionRequirePinnedHelm(t)
	first := newRegressionChartRepo(t, false, "")
	archives := map[string][]byte{
		"/other-1.0.0.tgz": regressionNamedPackage(t, "other", "1.0.0"),
		"/other-2.0.0.tgz": regressionNamedPackage(t, "other", "2.0.0"),
	}
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  other:\n    - name: other\n      version: 2.0.0\n      urls: [other-2.0.0.tgz]\n    - name: other\n      version: 1.0.0\n      urls: [other-1.0.0.tgz]\n")
			return
		}
		if body, ok := archives[r.URL.Path]; ok {
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(second.Close)
	first.extraEntries = fmt.Sprintf("  other:\n    - name: other\n      version: 1.0.0\n      urls: [%s/other-1.0.0.tgz]\n", second.URL)
	for _, name := range []string{"second", "first", managerKeyPrefix + managerKey(first.server.URL)} {
		for _, enabled := range []bool{false, true} {
			t.Run(name+strconv.FormatBool(enabled), func(t *testing.T) {
				dir := t.TempDir()
				chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: '>=1.0.0'\n    repository: '@first'\n  - name: other\n    version: '>=1.0.0'\n    repository: '@%s'\n", name)
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
				var store *IndexStore
				if enabled {
					store = NewIndexStore(t.TempDir(), time.Hour, 1<<20)
				}
				app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
					{Name: "first", Repo: first.server.URL, Creds: HelmCreds{}},
					{Name: name, Repo: second.URL, Creds: HelmCreds{}},
				}, false, "", "", "", false, false, store, false)
				require.NoError(t, err)
				t.Cleanup(app.Dispose)
				err = app.DependencyBuild(t.Context())
				if name == "first" {
					require.ErrorContains(t, err, "already exists")
					return
				}
				require.NoError(t, err)
				output, _, err := app.Template(&TemplateOpts{Name: "collision", Namespace: "default"})
				require.NoError(t, err)
				require.Contains(t, output, `version: "1.0.0"`)
				require.Contains(t, output, `version: "2.0.0"`, "the second repository must retain its own index")
			})
		}
	}
}

func writeLocalIndexChart(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 1.0.0\n", name)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "configmap.yaml"), []byte(fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n", name)), 0o600))
}

func TestIndexCacheRejectsHelmInvalidYAMLBeforePublication(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, enabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			fixture := newRegressionChartRepo(t, false, "")
			fixture.extraEntries = "unknown: true\n"
			var store *IndexStore
			if enabled {
				store = NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			}
			repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
			_, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, false)
			require.ErrorContains(t, err, "unknown")
			fixture.mu.Lock()
			fixture.extraEntries = ""
			fixture.mu.Unlock()
			for range 2 {
				app, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, false)
				require.NoError(t, err, "corrected origin bytes must not wait for invalid cached freshness")
				regressionRender(t, app, "1.0.0")
			}
			if enabled {
				requests, _ := fixture.snapshot()
				require.Len(t, requests, 2, "only valid responses can warm the next build")
			}
		})
	}
}

func TestIndexCacheRefreshesWarmVersionWithoutDownloadURL(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			archive := regressionPackage(t, "1.0.0")
			var ready atomic.Bool
			var requests, conditional atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/child-1.0.0.tgz" {
					_, _ = w.Write(archive)
					return
				}
				requests.Add(1)
				if r.Header.Get("If-None-Match") == `"missing"` {
					conditional.Add(1)
				}
				w.Header().Set("ETag", `"missing"`)
				urls := ""
				if ready.Load() {
					w.Header().Set("ETag", `"available"`)
					urls = "      urls: [child-1.0.0.tgz]\n"
				}
				_, _ = fmt.Fprintf(w, "apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n%s  unrelated: []\n", urls)
			}))
			t.Cleanup(origin.Close)
			repo := HelmRepository{Name: "fixture", Repo: origin.URL, Creds: HelmCreds{}}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			store.FilterEnabled = filtered
			_, err := store.Revalidate(t.Context(), repoIdentity(repo), regressionClient(origin.URL))
			require.NoError(t, err)
			ready.Store(true)
			for range 2 {
				app, err := NewHelmAppWithIndexStore(regressionParent(t, "@fixture", nil), []HelmRepository{repo}, false, "", "", "", false, false, store, false)
				require.NoError(t, err)
				t.Cleanup(app.Dispose)
				require.NoError(t, app.DependencyBuild(t.Context()))
				regressionRender(t, app, "1.0.0")
			}
			require.EqualValues(t, 2, requests.Load())
			require.EqualValues(t, 1, conditional.Load())
		})
	}
}

func TestIndexCacheIntegrationFileAndVendoredDependencies(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		dir := t.TempDir()
		writeLocalIndexChart(t, filepath.Join(dir, "local"), "local")
		writeLocalIndexChart(t, filepath.Join(dir, "charts", "vendored"), "vendored")
		parent := "apiVersion: v2\nname: mixed\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n    repository: '@web'\n  - name: local\n    version: 1.0.0\n    repository: 'file://local'\n  - name: vendored\n    version: 1.0.0\n    repository: ''\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(parent), 0o600))
		app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{{Name: "web", Repo: fixture.server.URL, Creds: HelmCreds{}}}, false, "", "", "", false, false, store, false)
		require.NoError(t, err)
		t.Cleanup(app.Dispose)
		require.NoError(t, app.DependencyBuild(t.Context()))
		output, _, err := app.Template(&TemplateOpts{Name: "mixed", Namespace: "default"})
		require.NoError(t, err)
		require.Equal(t, 3, strings.Count(output, "kind: ConfigMap"))
	}
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "file and vendored dependencies must never enter the HTTP provider")
}

func TestIndexCacheIntegrationMutualTLS(t *testing.T) {
	regressionRequirePinnedHelm(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "index-cache-client"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		BasicConstraintsValid: true, IsCA: true,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	require.NoError(t, err)
	clientCertificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	clientCA := x509.NewCertPool()
	clientCA.AddCert(clientCertificate)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	fixture := &regressionChartRepo{version: "1.0.0", packages: map[string][]byte{"1.0.0": regressionPackage(t, "1.0.0")}}
	fixture.server = httptest.NewUnstartedServer(http.HandlerFunc(fixture.serveHTTP))
	fixture.server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCA}
	fixture.server.StartTLS()
	t.Cleanup(fixture.server.Close)
	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw}), 0o600))
	repo := HelmRepository{Name: "private", Repo: fixture.server.URL, Creds: HelmCreds{CAPath: caPath, CertData: certData, KeyData: keyData}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		app, err := regressionBuild(t, regressionParent(t, "@private", nil), repo, store, false)
		require.NoError(t, err, "both Go index fetching and Helm archive downloading must authenticate with the client certificate")
		regressionRender(t, app, "1.0.0")
		home := app.(*helm).cmd.helmHome
		credentials, err := filepath.Glob(filepath.Join(home, "config", "repo-credential-*"))
		require.NoError(t, err)
		require.Len(t, credentials, 3, "client certificate, key and frozen repository CA must be private")
		for _, file := range credentials {
			info, err := os.Stat(file)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
		app.Dispose()
		_, err = os.Stat(home)
		require.ErrorIs(t, err, os.ErrNotExist, "disposing the Helm home must remove all credential files")
	}
	requests, archives := fixture.snapshot()
	require.Len(t, requests, 1, "the second authenticated build must be warm")
	require.Equal(t, 2, archives)
}

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
	regressionRequirePinnedHelm(t)
	dir := t.TempDir()
	writeChartPackage(t, dir, "child", "0.1.2")
	fr := &fixtureRepo{dir: dir}
	archive, err := osReadFile(filepath.Join(fr.dir, "child-0.1.2.tgz"))
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/child-0.1.2.tgz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
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

	chartYAML := fmt.Sprintf(`apiVersion: v2
name: parent
version: 0.1.0
dependencies:
  - name: child
    version: ">=0.1.0"
    repository: %s
`, repo.srv.URL)
	newParent := func() string {
		t.Helper()
		parent := t.TempDir()
		require.NoError(t, osWriteFile(filepath.Join(parent, "Chart.yaml"), []byte(chartYAML), 0o600))
		return parent
	}

	repos := []HelmRepository{{Name: "fixture", Repo: repo.srv.URL, Creds: HelmCreds{}}}

	// COLD build: no store entry → helm downloads the index (1 GET), build
	// succeeds, harvest persists the index. workDir is the chart directory.
	h1, err := NewHelmAppWithIndexStore(newParent(), repos, false, "", "", "", false, false, store, false)
	require.NoError(t, err)
	t.Cleanup(h1.Dispose)
	require.NoError(t, h1.DependencyBuild(context.Background()))
	assert.Equal(t, int32(1), repo.indexGets.Load(), "cold build downloads the index once")
	_, err = store.Get(repoIdentity(repos[0]))
	require.NoError(t, err, "harvest must persist the downloaded index")

	// WARM build in a COMPLETELY FRESH work dir: --skip-refresh + prewarmed
	// index → zero index downloads.
	h2, err := NewHelmAppWithIndexStore(newParent(), repos, false, "", "", "", false, false, store, false)
	require.NoError(t, err)
	t.Cleanup(h2.Dispose)
	require.NoError(t, h2.DependencyBuild(context.Background()))
	assert.Equal(t, int32(1), repo.indexGets.Load(), "warm build must not download index.yaml (helm dependency build --skip-refresh)")

	// Forced refresh: policy flag makes the build ignore the store and
	// contact the origin again.
	h3, err := NewHelmAppWithIndexStore(newParent(), repos, false, "", "", "", false, false, store, true)
	require.NoError(t, err)
	t.Cleanup(h3.Dispose)
	require.NoError(t, h3.DependencyBuild(context.Background()))
	assert.Equal(t, int32(2), repo.indexGets.Load(), "forced refresh must contact the origin unconditionally")
}

func osMkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func osWriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func TestIndexCachePreservesArchiveGetter(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, explicit := range []bool{false, true} {
		t.Run(strconv.FormatBool(explicit), func(t *testing.T) {
			dataHome := t.TempDir()
			t.Setenv("HELM_DATA_HOME", dataHome)
			plugins := filepath.Join(dataHome, "plugins")
			if explicit {
				plugins = t.TempDir()
				t.Setenv("HELM_PLUGINS", plugins)
			} else {
				t.Setenv("HELM_PLUGINS", "")
				require.NoError(t, os.Unsetenv("HELM_PLUGINS"))
			}
			dir := filepath.Join(plugins, "fixture-getter")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("name: fixture-getter\nversion: 0.1.0\ndownloaders:\n  - protocols: [fixture]\n    command: getter.sh\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "getter.sh"), []byte("#!/bin/sh\ncat \"$HELM_PLUGIN_DIR/child.tgz\"\n"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "child.tgz"), regressionPackage(t, "1.0.0"), 0o600))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n      apiVersion: v2\n      urls: [fixture://charts/child.tgz]\n")
			}))
			t.Cleanup(server.Close)
			repo := HelmRepository{Name: "charts", Repo: server.URL, Creds: HelmCreds{}}
			for _, cached := range []bool{false, true} {
				var store *IndexStore
				if cached {
					store = NewIndexStore(t.TempDir(), time.Hour, 1<<20)
				}
				app, err := regressionBuild(t, regressionParent(t, "@charts", nil), repo, store, false)
				require.NoError(t, err, "cached=%v explicit=%v", cached, explicit)
				regressionRender(t, app, "1.0.0")
			}
		})
	}
}

func TestIndexCacheRejectsEmptyConfiguredCA(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	ca := filepath.Join(t.TempDir(), "empty-ca.pem")
	require.NoError(t, os.WriteFile(ca, nil, 0o600))
	repo := HelmRepository{Name: "charts", Repo: fixture.server.URL, Creds: HelmCreds{CAPath: ca}}
	_, err := regressionBuild(t, regressionParent(t, "@charts", nil), repo, NewIndexStore(t.TempDir(), time.Hour, 1<<20), false)
	require.ErrorContains(t, err, "empty")
	fixture.mu.Lock()
	requests := len(fixture.requests)
	fixture.mu.Unlock()
	require.Zero(t, requests, "invalid trust must fail before prewarming contacts the origin")
	app, err := regressionBuild(t, regressionParent(t, "@charts", nil), repo, nil, false)
	require.NoError(t, err, "cache-disabled HTTP builds preserve their existing CA handling")
	regressionRender(t, app, "1.0.0")
}

func TestIndexCacheDoesNotDiscoverCheckoutGetters(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, mode := range []string{"empty", "missing-home", "relative"} {
		t.Run(mode, func(t *testing.T) {
			for _, name := range []string{"HELM_PLUGINS", "HELM_DATA_HOME", "XDG_DATA_HOME"} {
				t.Setenv(name, "")
				require.NoError(t, os.Unsetenv(name))
			}
			switch mode {
			case "empty":
				t.Setenv("HELM_PLUGINS", "")
			case "missing-home":
				t.Setenv("HOME", "")
			case "relative":
				t.Setenv("HELM_PLUGINS", "checkout-plugins")
			}
			parent := regressionParent(t, "@charts", nil)
			plugins := filepath.Join(parent, helmPluginsDirectory())
			dir := filepath.Join(plugins, "fixture-getter")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("name: fixture-getter\nversion: 0.1.0\ndownloaders:\n  - protocols: [fixture]\n    command: getter.sh\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "getter.sh"), []byte("#!/bin/sh\ntouch \"$HELM_PLUGIN_DIR/executed\"\ncat \"$HELM_PLUGIN_DIR/child.tgz\"\n"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "child.tgz"), regressionPackage(t, "1.0.0"), 0o600))
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n      apiVersion: v2\n      urls: [fixture://charts/child.tgz]\n")
			}))
			t.Cleanup(origin.Close)
			// First prove this exact getter and archive work when explicitly
			// trusted. Remove its marker before exercising checkout isolation.
			func() {
				t.Setenv("HELM_PLUGINS", plugins)
				app, err := regressionBuild(t, regressionParent(t, "@charts", nil), HelmRepository{Name: "charts", Repo: origin.URL, Creds: HelmCreds{}}, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
				require.NoError(t, err)
				regressionRender(t, app, "1.0.0")
				require.FileExists(t, filepath.Join(dir, "executed"))
				require.NoError(t, os.Remove(filepath.Join(dir, "executed")))
			}()
			// Restore this subtest's empty/relative setting after the control.
			switch mode {
			case "empty":
				t.Setenv("HELM_PLUGINS", "")
			case "missing-home":
				require.NoError(t, os.Unsetenv("HELM_PLUGINS"))
			case "relative":
				t.Setenv("HELM_PLUGINS", "checkout-plugins")
			}
			_, err := regressionBuild(t, parent, HelmRepository{Name: "charts", Repo: origin.URL, Creds: HelmCreds{}}, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
			require.Error(t, err, "a checkout getter must not supply the archive")
			_, err = os.Stat(filepath.Join(dir, "executed"))
			require.True(t, os.IsNotExist(err), "Helm must never execute the checkout getter")
		})
	}
}

func TestIndexCacheAliasesWithSameCredentialView(t *testing.T) {
	regressionRequirePinnedHelm(t)
	archive := regressionPackage(t, "1.0.0")
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, ok := r.BasicAuth()
		if !ok || (password != "first" && password != "second") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/index.yaml" {
			_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  child:\n    - apiVersion: v2\n      name: child\n      version: 1.0.0\n      urls: [child-1.0.0.tgz]\n")
			return
		}
		_, _ = w.Write(archive)
	}))
	t.Cleanup(origin.Close)
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    alias: first-child\n    version: 1.0.0\n    repository: '@first'\n  - name: child\n    alias: second-child\n    version: 1.0.0\n    repository: '@second'\n"), 0o600))
			store := NewIndexStore(t.TempDir(), time.Minute, 1<<20)
			store.FilterEnabled = filtered
			app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
				{Name: "first", Repo: origin.URL, Creds: HelmCreds{Username: "reader", Password: "first"}},
				{Name: "second", Repo: origin.URL, Creds: HelmCreds{Username: "reader", Password: "first"}},
			}, false, "", "", "", false, false, store, false)
			require.NoError(t, err)
			t.Cleanup(app.Dispose)
			require.NoError(t, app.DependencyBuild(t.Context()))
			out, _, err := app.Template(&TemplateOpts{Name: "aliases", Namespace: "default"})
			require.NoError(t, err)
			require.Equal(t, 2, strings.Count(out, "kind: ConfigMap"))
		})
	}
}

func TestIndexCacheAliasesWithDifferentValidCredentials(t *testing.T) {
	regressionRequirePinnedHelm(t)
	archives := map[string][]byte{"first": regressionNamedPackage(t, "first", "1.0.0"), "second": regressionNamedPackage(t, "second", "1.0.0")}
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, password, ok := r.BasicAuth()
		if !ok || archives[password] == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/index.yaml" {
			_, _ = fmt.Fprintf(w, "apiVersion: v1\nentries:\n  %s:\n    - apiVersion: v2\n      name: %s\n      version: 1.0.0\n      urls: [%s-1.0.0.tgz]\n", password, password, password)
			return
		}
		if r.URL.Path != "/"+password+"-1.0.0.tgz" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write(archives[password])
	}))
	t.Cleanup(origin.Close)
	for _, secondURL := range []string{origin.URL, origin.URL + "/staging/../"} {
		t.Run(secondURL, func(t *testing.T) {
			for _, filtered := range []bool{false, true} {
				t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
					dir := t.TempDir()
					require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: first\n    version: 1.0.0\n    repository: '@first'\n  - name: second\n    version: 1.0.0\n    repository: '@second'\n"), 0o600))
					store := NewIndexStore(t.TempDir(), time.Minute, 1<<20)
					store.FilterEnabled = filtered
					app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
						{Name: "first", Repo: origin.URL, Creds: HelmCreds{Username: "reader", Password: "first"}},
						{Name: "second", Repo: secondURL, Creds: HelmCreds{Username: "reader", Password: "second"}},
					}, false, "", "", "", false, false, store, false)
					require.NoError(t, err)
					t.Cleanup(app.Dispose)
					err = app.DependencyBuild(t.Context())
					require.ErrorContains(t, err, "configure distinct repository URLs")
					require.Zero(t, requests.Load(), "reject ambiguous views before fetching an index or archive")
				})
			}
		})
	}
}

func TestIndexCacheUnregisteredRepositoryStillResolves(t *testing.T) {
	regressionRequirePinnedHelm(t)
	registered := newRegressionChartRepo(t, true, "")
	archive := regressionNamedPackage(t, "other", "1.0.0")
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  other:\n    - apiVersion: v2\n      name: other\n      version: 1.0.0\n      urls: [other-1.0.0.tgz]\n")
			return
		}
		_, _ = w.Write(archive)
	}))
	t.Cleanup(public.Close)
	dir := t.TempDir()
	chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n    repository: '@registered'\n  - name: other\n    version: 1.0.0\n    repository: %q\n", public.URL)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
	app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{{Name: "registered", Repo: registered.server.URL, Creds: HelmCreds{InsecureSkipVerify: true}}}, false, "", "", "127.0.0.1,localhost", false, false, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.NoError(t, app.DependencyBuild(t.Context()))
	require.Nil(t, app.(*helm).cmd.persistentIndexStore, "unmanaged HTTP dependencies preserve Helm refresh")
	output, _, err := app.Template(&TemplateOpts{Name: "triage", Namespace: "default"})
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(output, "kind: ConfigMap"))
}

func TestIndexCacheRespectsIgnoredRequirements(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	for _, filtered := range []bool{false, true} {
		t.Run(strconv.FormatBool(filtered), func(t *testing.T) {
			dir := regressionParent(t, "@fixture", nil)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte("invalid: ["), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".helmignore"), []byte("/requirements.yaml\n"), 0o600))
			store := NewIndexStore(t.TempDir(), time.Minute, 1<<20)
			store.FilterEnabled = filtered
			app, err := regressionBuild(t, dir, HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}, store, false)
			require.NoError(t, err)
			require.NotNil(t, app.(*helm).cmd.persistentIndexStore)
			regressionRender(t, app, "1.0.0")
		})
	}
}

func TestIndexCacheOversizedMetadataUsesHelmValidation(t *testing.T) {
	regressionRequirePinnedHelm(t)
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.NoError(t, file.Truncate(maxChartMetadataBytes+1))
	require.NoError(t, file.Close())
	app, err := NewHelmAppWithIndexStore(dir, nil, false, "", "", "", false, false, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.Nil(t, app.(*helm).cmd.persistentIndexStore, "optional prewarming must leave oversized metadata to Helm")
	require.ErrorContains(t, app.DependencyBuild(t.Context()), "maximum decompressed size")
}

func TestHelmRepositoryURLKeyMatchesPinnedHelm(t *testing.T) {
	for _, pair := range [][2]string{
		{"https://charts.example/charts", "https://charts.example/staging/../charts"},
		{"https://charts.example", "https://charts.example/"},
		{"https://charts.example/charts/", "https://charts.example/charts"},
	} {
		require.Equal(t, helmRepositoryURLKey(pair[0]), helmRepositoryURLKey(pair[1]))
	}
	for _, pair := range [][2]string{
		{"https://charts.example/charts", "https://charts.example/other"},
		{"https://charts.example/charts?view=one", "https://charts.example/charts?view=two"},
		{"https://charts.example/a%2Fb", "https://charts.example/a/b"},
	} {
		require.NotEqual(t, helmRepositoryURLKey(pair[0]), helmRepositoryURLKey(pair[1]))
	}
}

func TestIndexCacheKeepsCredentialProvidersInCallerSlice(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			provider := &HelmCreds{Username: "reader", Password: "first"}
			repos := []HelmRepository{{Name: "first", Repo: "https://charts.example/charts", Creds: provider}}
			if fail {
				repos = append(repos, HelmRepository{Name: "conflict", Repo: repos[0].Repo, Creds: HelmCreds{Password: "other"}})
			}
			app, err := NewHelmApp(t.TempDir(), repos, false, "", "", "", false, false)
			require.NoError(t, err)
			t.Cleanup(app.Dispose)
			err = app.(*helm).validateIndexRepositoryViews(t.Context())
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Same(t, provider, repos[0].Creds)
			provider.Password = "rotated"
			retained, err := app.(*helm).repos[0].GetPassword()
			require.NoError(t, err)
			require.Equal(t, "first", retained)
			next, err := NewHelmApp(t.TempDir(), repos[:1], false, "", "", "", false, false)
			require.NoError(t, err)
			t.Cleanup(next.Dispose)
			require.NoError(t, next.(*helm).validateIndexRepositoryViews(t.Context()))
			password, err := next.(*helm).repos[0].GetPassword()
			require.NoError(t, err)
			require.Equal(t, "rotated", password)
		})
	}
}

func TestIndexCacheUsesEffectiveRequirementsDependencies(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		dir := regressionParent(t, "https://unregistered.example/charts", nil)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte("dependencies:\n  - name: child\n    version: '>=1.0.0 <3.0.0'\n    repository: '@fixture'\n"), 0o600))
		app, err := regressionBuild(t, dir, repo, store, false)
		require.NoError(t, err)
		require.NotNil(t, app.(*helm).cmd.persistentIndexStore)
		regressionRender(t, app, "1.0.0")
	}
	requests, archives := fixture.snapshot()
	require.Len(t, requests, 1, "a clean warm build must not fetch the superseded Chart.yaml dependency")
	require.Equal(t, 2, archives)
	// The reverse override must still keep Helm's refresh for an unmanaged URL.
	dir := regressionParent(t, "@fixture", nil)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte("dependencies:\n  - name: child\n    version: 1.0.0\n    repository: https://unregistered.example/charts\n"), 0o600))
	app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{repo}, false, "", "", "", false, false, store, false)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.Nil(t, app.(*helm).cmd.persistentIndexStore)
}

func TestIndexCacheWarmVersionMissRevalidatesOnlyAffectedRepository(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, filtered := range []bool{false, true} {
		for _, newChart := range []bool{false, true} {
			for _, locked := range []bool{false, true} {
				t.Run(fmt.Sprintf("filtered-%t/locked-%t/new-chart-%t", filtered, locked, newChart), func(t *testing.T) {
					first := newRegressionChartRepo(t, false, "")
					first.mu.Lock()
					first.extraEntries = "  unrelated:\n    - apiVersion: v2\n      name: unrelated\n      version: 1.0.0\n      urls: [unused.tgz]\n"
					first.mu.Unlock()
					otherArchive := regressionNamedPackage(t, "other", "1.0.0")
					futureArchive := regressionNamedPackage(t, "future", "2.0.0")
					var secondRequests atomic.Int32
					second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/index.yaml" {
							secondRequests.Add(1)
							_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  other:\n    - apiVersion: v2\n      name: other\n      version: 1.0.0\n      urls: [other-1.0.0.tgz]\n")
							return
						}
						if r.URL.Path == "/future-2.0.0.tgz" {
							_, _ = w.Write(futureArchive)
							return
						}
						_, _ = w.Write(otherArchive)
					}))
					t.Cleanup(second.Close)
					repos := []HelmRepository{{Name: "first", Repo: first.server.URL, Creds: HelmCreds{}}, {Name: "second", Repo: second.URL, Creds: HelmCreds{}}}

					store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
					store.FilterEnabled = filtered
					parent := func(version string, names ...string) string {
						name := "child"
						if len(names) > 0 {
							name = names[0]
						}
						dir := t.TempDir()
						chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: %s\n    alias: first-child\n    version: %s\n    repository: '@first'\n  - name: other\n    alias: second-child\n    version: 1.0.0\n    repository: '@second'\n", name, version)
						require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
						return dir
					}
					build := func(dir string, cache *IndexStore) Helm {
						app, err := NewHelmAppWithIndexStore(dir, repos, false, "", "", "127.0.0.1,localhost", false, false, cache, false)
						require.NoError(t, err)
						t.Cleanup(app.Dispose)
						ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
						defer cancel()
						require.NoError(t, app.DependencyBuild(ctx))
						return app
					}
					build(parent("1.0.0"), store)
					first.publish("2.0.0")
					requestedName := "child"
					if newChart {
						requestedName = "future"
						first.mu.Lock()
						first.extraEntries += fmt.Sprintf("  future:\n    - apiVersion: v2\n      name: future\n      version: 2.0.0\n      urls: [%s/future-2.0.0.tgz]\n", second.URL)
						first.mu.Unlock()
					}
					dir := parent("2.0.0", requestedName)
					if locked {
						build(dir, nil)
						require.NoError(t, os.RemoveAll(filepath.Join(dir, "charts")))
					}
					beforeFirst, _ := first.snapshot()
					beforeSecond := secondRequests.Load()
					app := build(dir, store)
					out, _, err := app.Template(&TemplateOpts{Name: "versions", Namespace: "default"})
					require.NoError(t, err)
					require.Contains(t, out, "version: \"2.0.0\"")
					require.Contains(t, out, "version: \"1.0.0\"")
					afterFirst, _ := first.snapshot()
					afterSecond := secondRequests.Load()
					require.Len(t, afterFirst, len(beforeFirst)+1)
					require.Equal(t, beforeSecond, afterSecond)
					require.Equal(t, `"1.0.0"`, afterFirst[len(afterFirst)-1].etag, "a warm version miss must retain conditional validators")
					build(parent("2.0.0", requestedName), store)
					warmFirst, _ := first.snapshot()
					warmSecond := secondRequests.Load()
					require.Len(t, warmFirst, len(afterFirst))
					require.Equal(t, afterSecond, warmSecond)
				})
			}
		}
	}
}

func TestIndexCacheWarmVersionMissRetriesOnlyOnce(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	_, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, false)
	require.NoError(t, err)
	dir := regressionParent(t, "@fixture", nil)
	chart, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	chart = bytes.ReplaceAll(chart, []byte("'>=1.0.0 <3.0.0'"), []byte("2.0.0"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), chart, 0o600))
	_, err = regressionBuild(t, dir, repo, store, false)
	require.Error(t, err)
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 2)
	require.Equal(t, http.StatusNotModified, requests[1].status)
}
