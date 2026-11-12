package helm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexBuildOriginFailureDoesNotFallBack(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "hard refresh"}[forced], func(t *testing.T) {
			var requests, commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusUnauthorized) }))
			t.Cleanup(server.Close)
			command, err := newCmdWithVersion(t.TempDir(), false, "", "", func(*exec.Cmd, func(string) string) (string, error) { commands.Add(1); return "", nil })
			require.NoError(t, err)
			t.Cleanup(command.Close)
			repo := HelmRepository{Name: "private", Repo: server.URL, Creds: HelmCreds{}}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			require.NoError(t, store.Set(repoIdentity(repo), []byte(regressionIndexBody)))
			old := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(store.cacheFileName(repoIdentity(repo)), old, old))
			command.persistentIndexStore = store
			app := &helm{cmd: *command, repos: []HelmRepository{repo}, forceRefresh: forced}
			err = app.DependencyBuild(t.Context())
			require.ErrorContains(t, err, "401")
			require.EqualValues(t, 1, requests.Load())
			require.Zero(t, commands.Load(), "origin failure must not be hidden by a second download through Helm")
		})
	}
}

func TestIndexBuildUnsupportedGetterUsesWholeLegacyPath(t *testing.T) {
	t.Setenv("HELM_DATA_HOME", "/inherited/getter-plugins")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: plugins\nversion: 1.0.0\ndependencies:\n  - name: custom\n    version: 1.0.0\n    repository: s3://custom-getter/charts\n"), 0o600))
	var calls [][]string
	var environment []string
	command, err := newCmdWithVersion(dir, false, "", "", func(command *exec.Cmd, _ func(string) string) (string, error) {
		calls = append(calls, append([]string(nil), command.Args...))
		environment = command.Env
		return "", nil
	})
	require.NoError(t, err)
	t.Cleanup(command.Close)
	command.persistentIndexStore = NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	app := &helm{cmd: *command, repos: []HelmRepository{{Name: "http", Repo: server.URL, Creds: HelmCreds{}}}}
	require.NoError(t, app.DependencyBuild(t.Context()))
	require.Zero(t, requests.Load(), "classification must happen before any provider fetch")
	require.Len(t, calls, 2)
	require.Contains(t, strings.Join(calls[0], " "), "repo add")
	require.Contains(t, strings.Join(calls[1], " "), "dependency build")
	require.NotContains(t, calls[1], "--skip-refresh")
	require.Contains(t, environment, "HELM_DATA_HOME=/inherited/getter-plugins", "unsupported getters must retain their legacy plugin environment")
}

func TestIndexBuildDeduplicatesForcedAliases(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: child\n    alias: first\n    version: 1.0.0\n    repository: '@first'\n  - name: child\n    alias: second\n    version: 1.0.0\n    repository: 'alias:second'\n"), 0o600))
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
		{Name: "first", Repo: fixture.server.URL, Creds: HelmCreds{}},
		{Name: "second", Repo: fixture.server.URL + "/", Creds: HelmCreds{}},
	}, false, "", "", "", false, false, store, true)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.NoError(t, app.DependencyBuild(t.Context()))
	output, _, err := app.Template(&TemplateOpts{Name: "aliases", Namespace: "default"})
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(output, "kind: ConfigMap"))
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "aliases and trailing-slash variants share one forced fetch")
	require.Empty(t, requests[0].etag)
}

func TestIndexBuildIsolatesInheritedHelmHomes(t *testing.T) {
	regressionRequirePinnedHelm(t)
	shared := t.TempDir()
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "HELM_REPOSITORY_CACHE"} {
		t.Setenv(name, shared)
	}
	t.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(shared, "repositories.yaml"))
	t.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(shared, "registry.json"))
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	app, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, NewIndexStore(t.TempDir(), time.Hour, 1<<20), false)
	require.NoError(t, err)
	regressionRender(t, app, "1.0.0")
	files, err := os.ReadDir(shared)
	require.NoError(t, err)
	require.Empty(t, files, "enabled builds must never write config, indexes or credentials into inherited Helm homes")
}
