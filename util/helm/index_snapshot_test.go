package helm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func snapshotFixtureIndex(padding int) []byte {
	return []byte("apiVersion: v1\nentries:\n" +
		"  child:\n" +
		"    - apiVersion: v2\n      name: child\n      version: 2.0.0\n      urls: [child-2.0.0.tgz]\n      annotations: {fixture: preserved}\n" +
		"    - apiVersion: v2\n      name: child\n      version: 1.0.0\n      urls: [child-1.0.0.tgz]\n" +
		"  sibling:\n    - apiVersion: v2\n      name: sibling\n      version: 1.0.0\n      urls: [sibling-1.0.0.tgz]\n" +
		"  unrelated:\n    - apiVersion: v2\n      name: unrelated\n      version: 1.0.0\n      urls: [unrelated.tgz]\n      description: " + strings.Repeat("x", padding) + "\n")
}

func TestIndexSnapshotPreservesSelectedFieldsAndVersions(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	full := snapshotFixtureIndex(1024)
	snapshot, filtered, err := cache.snapshot(full, map[string]struct{}{"child": {}, "sibling": {}})
	require.NoError(t, err)
	require.True(t, filtered)
	require.True(t, json.Valid(snapshot), "pinned Helm has a JSON fast path")
	var index struct {
		APIVersion string
		Entries    map[string][]struct {
			APIVersion  string
			Version     string
			URLs        []string
			Annotations map[string]string
		}
	}
	require.NoError(t, json.Unmarshal(snapshot, &index))
	require.Equal(t, "v1", index.APIVersion)
	require.Len(t, index.Entries, 2)
	require.Len(t, index.Entries["child"], 2, "keep all versions for Helm constraints and lockfiles")
	require.Equal(t, "v2", index.Entries["child"][0].APIVersion)
	require.Equal(t, []string{"child-2.0.0.tgz"}, index.Entries["child"][0].URLs)
	require.Equal(t, "preserved", index.Entries["child"][0].Annotations["fixture"])
	require.NotContains(t, index.Entries, "unrelated")
	_, _, err = cache.snapshot(full, map[string]struct{}{"missing": {}})
	require.ErrorContains(t, err, "absent")
	unchanged, filtered, err := cache.snapshot(full, map[string]struct{}{"child": {}, "sibling": {}, "unrelated": {}})
	require.NoError(t, err)
	require.False(t, filtered)
	require.Equal(t, full, unchanged, "do not transform indexes when every chart is needed")
}

func TestIndexSnapshotCacheBoundsRetainedBytes(t *testing.T) {
	cache := newIndexSnapshotCache(4096)
	for i := range 20 {
		body := append(snapshotFixtureIndex(64), []byte(fmt.Sprintf("# generation %d\n", i))...)
		_, err := cache.projection(body)
		require.NoError(t, err)
		require.LessOrEqual(t, cache.used, cache.limit)
	}
	require.Less(t, cache.entries.Len(), 20, "old body generations must be evicted")
	_, err := cache.projection(snapshotFixtureIndex(8192))
	require.ErrorContains(t, err, "budget")
	_, err = cache.projection([]byte("apiVersion: v1\napiVersion: v1\nentries: {}\n"))
	require.Error(t, err, "unsafe YAML conversion must fall back to the original full index")
}

func TestIndexSnapshotConcurrentDependencySets(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	body := snapshotFixtureIndex(128 << 10)
	var workers sync.WaitGroup
	results := make(chan error, 16)
	for i := range 16 {
		workers.Go(func() {
			name := []string{"child", "sibling"}[i%2]
			data, _, err := cache.snapshot(body, map[string]struct{}{name: {}})
			if err == nil {
				var parsed struct{ Entries map[string]json.RawMessage }
				err = json.Unmarshal(data, &parsed)
				if err == nil && (len(parsed.Entries) != 1 || parsed.Entries[name] == nil) {
					err = errors.New("concurrent snapshot returned another app's dependency set")
				}
			}
			results <- err
		})
	}
	workers.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
}

func TestIndexSnapshotRetryClassification(t *testing.T) {
	for _, message := range []string{
		"no cached repository for fixture found: error loading index.yaml",
		"child chart not found in repo https://charts.example",
		"can't get a valid version for subchart child (https://charts.example)",
		"failed running helm: Error: could not find : entry not found",
	} {
		require.True(t, isIndexSnapshotError(errors.New(message)))
	}
	for _, message := range []string{
		"could not download chart.tgz: 401 Unauthorized",
		"failed to fetch chart.tgz: connection timed out",
		"failed to fetch chart.tgz: could not find object: entry not found",
		"can't get a valid version for registry-child (oci://registry.example)",
		"can't get a valid version for local-child (file://../child)",
		"Chart.lock is out of sync with Chart.yaml",
	} {
		require.False(t, isIndexSnapshotError(errors.New(message)))
	}
}

func TestIndexSnapshotGroupsOriginalNamesByRepository(t *testing.T) {
	for _, metadata := range []struct {
		apiVersion string
		legacy     bool
	}{{"v2", false}, {"v1", true}, {"", true}, {"v2", true}} {
		t.Run(fmt.Sprintf("api=%s/requirements=%t", metadata.apiVersion, metadata.legacy), func(t *testing.T) {
			dir := t.TempDir()
			dependencies := "dependencies:\n  - name: child\n    alias: renamed\n    repository: '@first'\n  - name: sibling\n    repository: https://charts.example/repo/\n  - name: child\n    alias: another\n    repository: 'alias:second'\n  - name: local\n    repository: file://../local\n  - name: registry\n    repository: oci://registry.example/charts\n"
			chart := "apiVersion: " + metadata.apiVersion + "\n" + dependencies
			if metadata.legacy {
				chart = "apiVersion: " + metadata.apiVersion + "\ndependencies: []\n"
				require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte(dependencies), 0o600))
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
			names, err := dependencyNamesByRepository(t.Context(), dir, []HelmRepository{
				{Name: "first", Repo: "https://charts.example/repo"},
				{Name: "second", Repo: "https://charts.example/repo/"},
			})
			require.NoError(t, err)
			require.Len(t, names, 1)
			require.Equal(t, map[string]struct{}{"child": {}, "sibling": {}}, names[scopedIndexIdentity("https://charts.example/repo", "", "", "")])
		})
	}
}

func TestIndexSnapshotRealHelmAndFullBackup(t *testing.T) {
	regressionRequirePinnedHelm(t)
	full := snapshotFixtureIndex(128 << 10)
	packages := map[string][]byte{
		"/child-1.0.0.tgz":   regressionPackage(t, "1.0.0"),
		"/child-2.0.0.tgz":   regressionPackage(t, "2.0.0"),
		"/sibling-1.0.0.tgz": regressionNamedPackage(t, "sibling", "1.0.0"),
	}
	for _, mode := range []string{"full", "filtered", "projection-fallback", "fallback", "invalid-number", "fallback-fails", "archive-error"} {
		t.Run(mode, func(t *testing.T) {
			full := full
			if mode == "projection-fallback" {
				// Helm accepts case-insensitive struct fields. An unsupported
				// projection must preserve that valid original representation.
				full = []byte(strings.Replace(string(full), "apiVersion: v1", "APIVersion: v1", 1))
			}
			if mode == "invalid-number" {
				// Both original YAML and projected JSON must reject invalid
				// numeric chart versions; fallback must not hide source errors.
				full = []byte(strings.ReplaceAll(string(full), "version: 1.0.0", "version: 1.0"))
			}
			var indexRequests, archiveRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/index.yaml" {
					indexRequests.Add(1)
					_, _ = w.Write(full)
					return
				}
				if body, ok := packages[r.URL.Path]; ok {
					archiveRequests.Add(1)
					if mode == "archive-error" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					_, _ = w.Write(body)
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(server.Close)
			dir := t.TempDir()
			chart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    alias: first\n    version: '>=1.0.0 <3.0.0'\n    repository: '@one'\n  - name: sibling\n    version: 1.0.0\n    repository: %q\n  - name: child\n    alias: second\n    version: 1.0.0\n    repository: 'alias:two'\n", server.URL+"/")
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			store.FilterEnabled = mode != "full"
			app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
				{Name: "one", Repo: server.URL, Creds: HelmCreds{}},
				{Name: "two", Repo: server.URL + "/", Creds: HelmCreds{}},
			}, false, "", "", "127.0.0.1,localhost", false, false, store, false)
			require.NoError(t, err)
			t.Cleanup(app.Dispose)
			h := app.(*helm)
			original := h.cmd.runWithRedactor
			builds := 0
			h.cmd.runWithRedactor = func(command *exec.Cmd, redactor func(string) string) (string, error) {
				if len(command.Args) >= 3 && command.Args[1] == "dependency" && command.Args[2] == "build" {
					builds++
					cacheDir := filepath.Join(h.cmd.helmHome, "cache", "helm", "repository")
					files, err := filepath.Glob(filepath.Join(cacheDir, "*-index.yaml"))
					require.NoError(t, err)
					require.NotEmpty(t, files)
					for _, file := range files {
						body, err := os.ReadFile(file)
						require.NoError(t, err)
						if builds == 1 && mode != "full" && mode != "projection-fallback" {
							require.True(t, json.Valid(body))
							require.Less(t, len(body), 4096)
							require.NotContains(t, string(body), "unrelated")
						} else {
							require.Equal(t, full, body)
						}
						if mode == "fallback-fails" || mode == "fallback" && builds == 1 {
							require.NoError(t, os.WriteFile(file, []byte("invalid: ["), 0o600))
						}
					}
				}
				return original(command, redactor)
			}
			err = app.DependencyBuild(t.Context())
			switch mode {
			case "fallback-fails", "invalid-number":
				require.ErrorContains(t, err, "complete-index fallback failed")
				require.Equal(t, 2, builds)
			case "archive-error":
				require.ErrorContains(t, err, "401")
				require.Equal(t, 1, builds, "archive authentication failures must not trigger a snapshot retry")
			default:
				require.NoError(t, err)
				wantBuilds := 1
				if mode == "fallback" {
					wantBuilds = 2
				}
				require.Equal(t, wantBuilds, builds)
				output, _, err := app.Template(&TemplateOpts{Name: "snapshot", Namespace: "default"})
				require.NoError(t, err)
				require.Equal(t, 3, strings.Count(output, "kind: ConfigMap"))
				require.EqualValues(t, 3, archiveRequests.Load())
			}
			require.EqualValues(t, 1, indexRequests.Load(), "fallback must reuse the full generation without another origin GET")
			identity := scopedIndexIdentity(server.URL, credsScope(HelmCreds{}), "", "127.0.0.1,localhost")
			stored, err := store.Get(identity)
			require.NoError(t, err)
			require.Equal(t, full, stored, "persistent storage must retain complete original bytes")
		})
	}
}

func TestIndexSnapshotExpiryHardRefreshAndLockfile(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	fixture.mu.Lock()
	fixture.extraEntries = "  unrelated:\n    - apiVersion: v2\n      name: unrelated\n      version: 1.0.0\n      urls: [unused.tgz]\n"
	fixture.mu.Unlock()
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	store.FilterEnabled = true
	firstDir := regressionParent(t, "@fixture", nil)
	first, err := regressionBuild(t, firstDir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, first, "1.0.0")
	lock, err := os.ReadFile(filepath.Join(firstDir, "Chart.lock"))
	require.NoError(t, err)
	fixture.publish("2.0.0")
	identity := scopedIndexIdentity(repo.Repo, credsScope(repo.Creds), "", "127.0.0.1,localhost")
	regressionExpire(t, store, identity)
	changed, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, changed, "2.0.0")
	lockedDir := regressionParent(t, "@fixture", lock)
	locked, err := regressionBuild(t, lockedDir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, locked, "1.0.0")
	actualLock, err := os.ReadFile(filepath.Join(lockedDir, "Chart.lock"))
	require.NoError(t, err)
	require.Equal(t, lock, actualLock)
	regressionExpire(t, store, identity)
	unchanged, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, false)
	require.NoError(t, err)
	regressionRender(t, unchanged, "2.0.0")
	hard, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, store, true)
	require.NoError(t, err)
	regressionRender(t, hard, "2.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 4)
	require.Equal(t, []int{200, 200, 304, 200}, []int{requests[0].status, requests[1].status, requests[2].status, requests[3].status})
	require.Empty(t, requests[3].etag, "hard refresh remains unconditional when filtering is enabled")
}

func TestIndexSnapshotLockedBuildRestoresFullBackup(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	fixture.mu.Lock()
	fixture.extraEntries = "  unrelated:\n    - apiVersion: v2\n      name: unrelated\n      version: 1.0.0\n      urls: [unused.tgz]\n"
	fixture.mu.Unlock()
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	store.FilterEnabled = true
	firstDir := regressionParent(t, "@fixture", nil)
	_, err := regressionBuild(t, firstDir, repo, store, false)
	require.NoError(t, err)
	lock, err := os.ReadFile(filepath.Join(firstDir, "Chart.lock"))
	require.NoError(t, err)
	for name, corruption := range map[string][]byte{"malformed": []byte("invalid: ["), "missing-entry": []byte("apiVersion: v1\nentries: {}\n")} {
		t.Run(name, func(t *testing.T) {
			app, err := NewHelmAppWithIndexStore(regressionParent(t, "@fixture", lock), []HelmRepository{repo}, false, "", "", "127.0.0.1,localhost", false, false, store, false)
			require.NoError(t, err)
			t.Cleanup(app.Dispose)
			h := app.(*helm)
			original := h.cmd.runWithRedactor
			builds := 0
			h.cmd.runWithRedactor = func(command *exec.Cmd, redactor func(string) string) (string, error) {
				if len(command.Args) >= 3 && command.Args[1] == "dependency" && command.Args[2] == "build" {
					builds++
					if builds == 1 {
						files, err := filepath.Glob(filepath.Join(h.cmd.helmHome, "cache", "helm", "repository", "*-index.yaml"))
						require.NoError(t, err)
						require.NotEmpty(t, files)
						for _, file := range files {
							require.NoError(t, os.WriteFile(file, corruption, 0o600))
						}
					}
				}
				return original(command, redactor)
			}
			require.NoError(t, app.DependencyBuild(t.Context()))
			require.Equal(t, 2, builds, "locked builds must also restore the complete snapshot once")
			regressionRender(t, app, "1.0.0")
		})
	}

	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "fallback reuses the original generation without another index request")
}

func BenchmarkIndexSnapshotHelmBuild27MiB(b *testing.B) {
	regressionRequirePinnedHelm(b)
	full := append(benchmarkIndexBody(b), []byte("  child:\n    - apiVersion: v2\n      name: child\n      version: 1.0.0\n      urls: [child-1.0.0.tgz]\n")...)
	archive := regressionPackage(b, "1.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			_, _ = w.Write(full)
		case "/child-1.0.0.tgz":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	b.Cleanup(server.Close)
	for _, filtered := range []bool{false, true} {
		b.Run(map[bool]string{false: "full", true: "filtered"}[filtered], func(b *testing.B) {
			store := NewIndexStore(b.TempDir(), time.Hour, 32<<20)
			store.FilterEnabled = filtered
			repo := HelmRepository{Name: "fixture", Repo: server.URL, Creds: HelmCreds{}}
			require.NoError(b, store.Set(repoIdentity(repo), full))
			if filtered {
				_, _, err := dependencyIndexes.snapshot(full, map[string]struct{}{"child": {}})
				require.NoError(b, err)
			}
			var cpu time.Duration
			var copied int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for range b.N {
				dir := b.TempDir()
				chart := "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n    repository: '@fixture'\n"
				require.NoError(b, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
				app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{repo}, false, "", "", "", false, false, store, false)
				require.NoError(b, err)
				h := app.(*helm)
				original := h.cmd.runWithRedactor
				h.cmd.runWithRedactor = func(command *exec.Cmd, redactor func(string) string) (string, error) {
					out, err := original(command, redactor)
					if command.ProcessState != nil {
						cpu += command.ProcessState.UserTime() + command.ProcessState.SystemTime()
					}
					return out, err
				}
				b.StartTimer()
				err = app.DependencyBuild(b.Context())
				b.StopTimer()
				require.NoError(b, err)
				files, err := filepath.Glob(filepath.Join(h.cmd.helmHome, "cache", "helm", "repository", "*-index.yaml"))
				require.NoError(b, err)
				for _, file := range files {
					info, err := os.Stat(file)
					require.NoError(b, err)
					copied += info.Size()
				}
				app.Dispose()
			}
			b.ReportMetric(float64(cpu)/float64(time.Millisecond)/float64(b.N), "helm-cpu-ms/op")
			b.ReportMetric(float64(copied)/float64(b.N), "index-bytes/op")
		})
	}
}

func TestChartMetadataIgnoreRules(t *testing.T) {
	for _, test := range []struct {
		rule    string
		ignored bool
	}{
		{"requirements.yaml", true},
		{"/requirements.yaml", true},
		{"!Chart.yaml", true},
		{"!requirements.yaml", false},
		{"requirements.yaml/", false},
		{"templates/*", false},
		{"\ufeffrequirements.yaml", true},
	} {
		t.Run(test.rule, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte("dependencies: []\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".helmignore"), []byte(test.rule+"\n"), 0o600))
			_, err := ReadChartMetadata(dir, "requirements.yaml")
			if test.ignored {
				require.True(t, os.IsNotExist(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChartMetadataReaderBoundsAndHelmIgnore(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.yaml"), []byte("dependencies: []\n"), 0o600))
	for _, tc := range []struct {
		rule    string
		ignored bool
		invalid bool
	}{
		{"requirements.yaml\n", true, false},
		{"/requirements.yaml\n", true, false},
		{"*.yaml\n", true, false},
		{"*.tmp\n", false, false},
		{"requirements.yaml/\n", false, false},
		{"!Chart.yaml\n", true, false},
		{"!requirements.yaml\n", false, false},
		{"\ufeffrequirements.yaml\n", true, false},
		{"requirements.yaml\n**\n", false, true},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".helmignore"), []byte(tc.rule), 0o600))
			data, err := ReadChartMetadata(dir, "requirements.yaml")
			switch {
			case tc.invalid:
				require.Error(t, err)
			case tc.ignored:
				require.True(t, os.IsNotExist(err))
			default:
				require.NoError(t, err)
				require.Equal(t, "dependencies: []\n", string(data))
			}
		})
	}
	require.NoError(t, os.Remove(filepath.Join(dir, ".helmignore")))
	file, err := os.Create(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.NoError(t, file.Truncate(maxChartMetadataBytes+1))
	require.NoError(t, file.Close())
	_, err = ReadChartMetadata(dir, "Chart.yaml")
	require.ErrorContains(t, err, "oversized")
	app, err := NewHelmAppWithIndexStore(dir, nil, false, "", "", "", false, false, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.Nil(t, app.(*helm).cmd.persistentIndexStore, "optional prewarming must defer oversized metadata to Helm")
}
