package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

func TestHelmIndexCacheEnablement(t *testing.T) {
	temporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, temporary)
	}
	for _, test := range []struct {
		name, directory string
		enabled         bool
	}{
		{name: "disabled by default"},
		{name: "directory alone does not enable reuse", directory: filepath.Join(temporary, "disabled-override")},
		{name: "enabled with default directory", enabled: true},
		{name: "enabled with custom directory", enabled: true, directory: filepath.Join(temporary, "custom")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(metrics.NewMetricsServer(), nil, RepoServerInitConstants{
				HelmIndexCacheEnabled: test.enabled, HelmIndexCacheDir: test.directory, HelmRegistryMaxIndexSize: 1 << 20,
			}, &git.NoopCredsStore{}, t.TempDir())
			if !test.enabled {
				require.Nil(t, service.helmIndexStore)
				return
			}
			require.NotNil(t, service.helmIndexStore)
			directory := test.directory
			if directory == "" {
				directory = filepath.Join(temporary, "helm-index-cache")
			}
			require.NoError(t, service.helmIndexStore.Set("fixture", []byte("apiVersion: v1\nentries: {}\n")))
			records, err := filepath.Glob(filepath.Join(directory, "*.json"))
			require.NoError(t, err)
			require.Len(t, records, 1, "the configured directory must receive the index record")
		})
	}
}

func packageIndexTestChart(t *testing.T, version string) []byte {
	t.Helper()
	var result bytes.Buffer
	gzipWriter := gzip.NewWriter(&result)
	writer := tar.NewWriter(gzipWriter)
	for name, body := range map[string]string{
		"child/Chart.yaml":               fmt.Sprintf("apiVersion: v2\nname: child\nversion: %s\n", version),
		"child/templates/configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: dependency-version\ndata:\n  version: {{ .Chart.Version | quote }}\n",
	} {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}))
		_, err := writer.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, gzipWriter.Close())
	return result.Bytes()
}

func TestHelmIndexCacheManifestGeneration(t *testing.T) {
	t.Setenv("ARGOCD_GPG_ENABLED", "false")
	hack, err := filepath.Abs("../../hack")
	require.NoError(t, err)
	t.Setenv("PATH", hack+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("requires repository-pinned Helm")
	}
	version, err := exec.CommandContext(t.Context(), "helm", "version", "--short").Output()
	require.NoError(t, err)
	versions, err := os.ReadFile("../../hack/tool-versions.sh")
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(versions), "\n") {
		if pinned, ok := strings.CutPrefix(line, "HELM_VERSION="); ok {
			require.True(t, strings.HasPrefix(string(version), "v"+strings.Trim(pinned, "\"'")+"+"), "use the pinned Helm binary")
		}
	}
	archives := map[string][]byte{"1.0.0": packageIndexTestChart(t, "1.0.0"), "2.0.0": packageIndexTestChart(t, "2.0.0")}
	var mu sync.Mutex
	currentVersion := "1.0.0"
	requiredPassword := ""
	type indexRequest struct {
		conditional   string
		status, bytes int
	}
	var requests []indexRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/index.yaml" {
			for version, body := range archives {
				if r.URL.Path == "/child-"+version+".tgz" {
					_, _ = w.Write(body)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		if requiredPassword != "" {
			_, password, ok := r.BasicAuth()
			if !ok || password != requiredPassword {
				requests = append(requests, indexRequest{status: http.StatusUnauthorized})
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		etag := `"` + currentVersion + `"`
		w.Header().Set("ETag", etag)
		request := indexRequest{conditional: r.Header.Get("If-None-Match"), status: http.StatusOK}
		if request.conditional == etag {
			request.status = http.StatusNotModified
			requests = append(requests, request)
			w.WriteHeader(request.status)
			return
		}
		body := fmt.Sprintf("apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: %s\n      urls: [child-%s.tgz]\n", currentVersion, currentVersion)
		if currentVersion != "1.0.0" {
			body += "    - name: child\n      version: 1.0.0\n      urls: [child-1.0.0.tgz]\n"
		}
		request.bytes = len(body)
		requests = append(requests, request)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	remote := t.TempDir()
	initGitRepo(t, newGitRepoOptions{path: remote})
	for _, name := range []string{"cold", "warm", "expired", "hard", "revision", "locked"} {
		dir := filepath.Join(remote, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: child\n    version: '>=1.0.0'\n    repository: '@charts'\n"), 0o644))
	}
	legacyDir := filepath.Join(remote, "legacy")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "Chart.yaml"), []byte("apiVersion: v1\nname: legacy\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "requirements.yaml"), []byte("dependencies:\n  - name: child\n    version: '>=1.0.0'\n    repository: '@charts'\n"), 0o644))
	// Establish the disabled-path baseline and produce a real Chart.lock
	// with Helm's own digest calculation rather than fabricating a lockfile.
	baseline, err := helm.NewHelmApp(filepath.Join(remote, "locked"), []helm.HelmRepository{{Name: "charts", Repo: server.URL, Creds: helm.HelmCreds{}}}, false, "", "", "", false, false)
	require.NoError(t, err)
	t.Cleanup(baseline.Dispose)
	require.NoError(t, baseline.DependencyBuild(t.Context()))
	require.NoError(t, os.RemoveAll(filepath.Join(remote, "locked", "charts")))
	lockfile, err := os.ReadFile(filepath.Join(remote, "locked", "Chart.lock"))
	require.NoError(t, err)
	mu.Lock()
	baselineIndexGets := len(requests)
	requests = nil
	mu.Unlock()
	require.Positive(t, baselineIndexGets)
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Cache Test", "-c", "user.email=cache-test@example.invalid", "commit", "-m", "charts"}} {
		command := exec.CommandContext(t.Context(), "git", args...)
		command.Dir = remote
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	cacheMocks := newCacheMocksWithOpts(24*time.Hour, time.Hour, time.Second)
	t.Cleanup(cacheMocks.mockCache.StopRedisCallback)
	storeDir := t.TempDir()
	service := NewService(metrics.NewMetricsServer(), cacheMocks.cache, RepoServerInitConstants{
		HelmIndexCacheEnabled: true, HelmIndexCacheDir: storeDir, HelmRegistryMaxIndexSize: 1 << 20, HelmManifestMaxExtractedSize: 1 << 20,
	}, &git.NoopCredsStore{}, t.TempDir())
	require.NoError(t, service.Init())
	t.Cleanup(func() {
		_ = os.Chmod(service.rootDir, 0o700)
		for _, root := range service.gitRepoPaths.GetPaths() {
			_ = os.Chmod(root, 0o700)
		}
	})
	generateUsing := func(target *Service, name string, hard, revision bool) *apiclient.ManifestResponse {
		t.Helper()
		url := "file://" + remote
		response, err := target.GenerateManifest(t.Context(), &apiclient.ManifestRequest{
			Repo: &v1alpha1.Repository{Repo: url}, Revision: "HEAD", AppName: "cache-" + name,
			ApplicationSource: &v1alpha1.ApplicationSource{RepoURL: url, Path: name, TargetRevision: "HEAD", Helm: &v1alpha1.ApplicationSourceHelm{}},
			Repos:             []*v1alpha1.Repository{{Name: "charts", Type: "helm", Repo: server.URL}},
			Namespace:         "default", KubeVersion: "1.31", ProjectSourceRepos: []string{"*"},
			NoCache: hard, NoRevisionCache: revision,
		})
		require.NoError(t, err)
		require.Len(t, response.Manifests, 1)
		return response
	}
	generate := func(name string, hard, revision bool) *apiclient.ManifestResponse {
		t.Helper()
		return generateUsing(service, name, hard, revision)
	}
	cold := generate("cold", false, false)
	files, err := filepath.Glob(filepath.Join(storeDir, "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	identity := server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})
	record, err := cacheMocks.cache.GetHelmIndexRetainedRecord(identity)
	require.NoError(t, err)
	validatedAt, err := time.Parse(time.RFC3339Nano, record.StoredAt)
	require.NoError(t, err)
	require.NoError(t, os.Remove(files[0]))
	warm := generate("warm", false, false)
	require.Equal(t, cold.Manifests, warm.Manifests)
	mu.Lock()
	count := len(requests)
	mu.Unlock()
	require.Equal(t, 1, count, "another clean chart directory must reuse the index without a GET")
	files, err = filepath.Glob(filepath.Join(storeDir, "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	info, err := os.Stat(files[0])
	require.NoError(t, err)
	require.WithinDuration(t, validatedAt, info.ModTime(), time.Millisecond, "Redis-to-disk copies must preserve validation time")
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(files[0], old, old))
	record.StoredAt = old.UTC().Format(time.RFC3339Nano)
	require.NoError(t, cacheMocks.cache.SetHelmIndexRetainedRecord(identity, record))
	generate("expired", false, false)
	mu.Lock()
	currentVersion = "2.0.0"
	mu.Unlock()
	hard := generate("hard", true, false)
	require.Contains(t, hard.Manifests[0], "2.0.0")
	generate("revision", false, true)
	legacy := generate("legacy", false, false)
	require.Contains(t, legacy.Manifests[0], "2.0.0")
	locked := generate("locked", false, false)
	require.Contains(t, locked.Manifests[0], "1.0.0", "a newer index must not unlock an existing Chart.lock")
	repoRoot := service.gitRepoPaths.GetPathIfExists(git.NormalizeGitURL("file://" + remote))
	closer := directoryPermissionInitializer(repoRoot)
	actualLockfile, err := os.ReadFile(filepath.Join(repoRoot, "locked", "Chart.lock"))
	require.NoError(t, err)
	require.Equal(t, lockfile, actualLockfile)
	require.NoError(t, closer.Close())
	// A hard refresh does not invent a dependency-build trigger when charts
	// and the existing marker already make templating succeed.
	unchanged := generate("cold", true, false)
	require.Equal(t, cold.Manifests, unchanged.Manifests)
	mu.Lock()
	got := append([]indexRequest(nil), requests...)
	mu.Unlock()
	require.Len(t, got, 4)
	require.Equal(t, []int{200, 304, 200, 200}, []int{got[0].status, got[1].status, got[2].status, got[3].status})
	require.Empty(t, got[0].conditional)
	require.Equal(t, `"1.0.0"`, got[1].conditional)
	require.Zero(t, got[1].bytes)
	require.Empty(t, got[2].conditional, "hard refresh must be unconditional")
	require.Empty(t, got[3].conditional, "forced revision resolution must be unconditional")
	disabled := NewService(metrics.NewMetricsServer(), cacheMocks.cache, RepoServerInitConstants{HelmIndexCacheDir: storeDir, HelmRegistryMaxIndexSize: 1 << 20, HelmManifestMaxExtractedSize: 1 << 20}, &git.NoopCredsStore{}, t.TempDir())
	require.Nil(t, disabled.helmIndexStore, "the default must retain the legacy path")
	require.NoError(t, disabled.Init())
	t.Cleanup(func() {
		_ = os.Chmod(disabled.rootDir, 0o700)
		for _, root := range disabled.gitRepoPaths.GetPaths() {
			_ = os.Chmod(root, 0o700)
		}
	})
	generateUsing(disabled, "warm", true, false)
	mu.Lock()
	disabledGets := len(requests) - len(got)
	mu.Unlock()
	require.Equal(t, baselineIndexGets, disabledGets, "disabled service behavior must match the measured legacy binary path")
	// Real always-fresh RPC entry points must still reach the origin when warm.
	repo := &v1alpha1.Repository{Repo: server.URL, Type: "helm"}
	_, err = service.GetHelmCharts(t.Context(), &apiclient.HelmChartsRequest{Repo: repo})
	require.NoError(t, err)
	_, err = service.TestRepository(t.Context(), &apiclient.TestRepositoryRequest{Repo: repo})
	require.NoError(t, err)
	resolved, err := service.ResolveRevision(t.Context(), &apiclient.ResolveRevisionRequest{Repo: repo, AmbiguousRevision: "*", App: &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Source: &v1alpha1.ApplicationSource{RepoURL: server.URL, Chart: "child"}}}})
	require.NoError(t, err)
	require.Equal(t, "2.0.0", resolved.Revision)
	_, err = service.GetRevisionChartDetails(t.Context(), &apiclient.RepoServerRevisionChartDetailsRequest{Repo: repo, Name: "child", Revision: "*"})
	require.NoError(t, err)
	mu.Lock()
	freshRequests := append([]indexRequest(nil), requests[len(got)+disabledGets:]...)
	currentVersion = "1.0.0"
	mu.Unlock()
	require.Len(t, freshRequests, 5, "four revalidations plus the separate helm pull index lookup")
	for _, request := range freshRequests[:4] {
		require.Equal(t, 304, request.status)
		require.Zero(t, request.bytes)
	}
	require.Equal(t, 200, freshRequests[4].status, "extraction remains outside dependency-index optimization")
	charts, err := service.GetHelmCharts(t.Context(), &apiclient.HelmChartsRequest{Repo: repo})
	require.NoError(t, err)
	require.Len(t, charts.Items, 1)
	require.Equal(t, []string{"1.0.0"}, charts.Items[0].Versions, "changed origin data must be immediately visible")
	mu.Lock()
	requiredPassword = "correct"
	mu.Unlock()
	_, err = service.TestRepository(t.Context(), &apiclient.TestRepositoryRequest{Repo: &v1alpha1.Repository{Repo: server.URL, Type: "helm", Username: "reader", Password: "wrong"}})
	require.ErrorContains(t, err, "401", "warm data must never hide invalid current credentials")
	t.Log("always-fresh RPCs: listing=304 connectivity=304 revision=304 chart-details=304 extraction=200 changed-listing=200 wrong-credentials=401")
	t.Logf("index requests: disabled=%d cold=1 Redis-warm=0 expired=1/304 hard=1/200 revision=1/200 locked=0 legacy=0 existing-build=0", disabledGets)
}
