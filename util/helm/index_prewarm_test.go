package helm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// repoConfigFor mirrors helm's repositories.yaml layout at
// <XDG_CONFIG_HOME>/helm/repositories.yaml, as produced by writeRepoEntries.
type repoConfigFor struct {
	APIVersion   string          `json:"apiVersion"`
	Generated    string          `json:"generated"`
	Repositories []helmRepoEntry `json:"repositories"`
}

func readRepoEntries(t *testing.T, cmd *Cmd) repoConfigFor {
	t.Helper()
	data, err := os.ReadFile(path.Join(cmd.helmHome, "config", "helm", "repositories.yaml"))
	require.NoError(t, err)
	var rf repoConfigFor
	require.NoError(t, yaml.Unmarshal(data, &rf))
	return rf
}

func newTestCmd(t *testing.T) *Cmd {
	t.Helper()
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestManagerKey(t *testing.T) {
	// helm's downloader.key is a hex sha256 of the URL; assert against the
	// independently computed value for a known input (with the prefix).
	assert.Equal(t, "helm-manager-b3649b7a1fb46221253664b51fc48ca5972e9ba4662cf05508a57f75c26bc67f", managerKeyPrefix+managerKey("http://127.0.0.1:18099/"))
	assert.Equal(t, "b3649b7a1fb46221253664b51fc48ca5972e9ba4662cf05508a57f75c26bc67f", managerKey("http://127.0.0.1:18099/"))
	assert.Len(t, managerKey("https://example.com/helm"), 64)
	// Distinct URLs map to distinct keys.
	assert.NotEqual(t, managerKey("https://a.example.com"), managerKey("https://b.example.com"))
	// Trailing slash changes the key (matches helm behavior).
	assert.NotEqual(t, managerKey("https://example.com"), managerKey("https://example.com/"))
}

func TestPrewarmIndexes(t *testing.T) {
	c := newTestCmd(t)
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
	c.persistentIndexStore = store
	indexData := []byte("apiVersion: v1\ngenerated: x\nentries: {}\n")

	require.NoError(t, c.prewarmIndexes("https://charts.example.com", indexData, "myrepo"))

	// Both the user-facing name and the helm-manager generated name must be
	// installed in the ephemeral cache home.
	cacheDir := path.Join(c.helmHome, "cache", "helm", "repository")
	for _, name := range []string{"myrepo", managerKeyPrefix + managerKey("https://charts.example.com")} {
		data, err := os.ReadFile(path.Join(cacheDir, name+"-index.yaml"))
		require.NoError(t, err, "expected prewarmed index for %s", name)
		assert.Equal(t, indexData, data)
	}
}

func TestPrewarmIndexesDisabledStore(t *testing.T) {
	c := newTestCmd(t)
	// A disabled store (empty dir) fails Get; callers fall back to RepoAdd.
	// prewarmIndexes itself is only called with a non-nil store and valid
	// data, so just assert the disabled store's CopyTo is inert.
	disabled := NewIndexStore("", time.Hour, 1<<30)
	c.persistentIndexStore = disabled
	err := c.prewarmIndexes("https://charts.example.com", []byte("data"), "myrepo")
	assert.Error(t, err, "CopyTo from a disabled store should fail")
}

func TestWriteRepoEntriesCreatesFile(t *testing.T) {
	c := newTestCmd(t)
	entries := []helmRepoEntry{
		{Name: "repo1", URL: "https://charts.example.com"},
		{Name: managerKeyPrefix + managerKey("https://charts.example.com"), URL: "https://charts.example.com"},
	}
	require.NoError(t, c.writeRepoEntries(entries))

	rf := readRepoEntries(t, c)
	assert.Equal(t, "v1", rf.APIVersion)
	require.Len(t, rf.Repositories, 2)
	assert.Equal(t, "repo1", rf.Repositories[0].Name)
	assert.Equal(t, "https://charts.example.com", rf.Repositories[0].URL)
}

func TestWriteRepoEntriesMergeIdempotent(t *testing.T) {
	c := newTestCmd(t)
	entries := []helmRepoEntry{{Name: "repo1", URL: "https://charts.example.com"}}
	require.NoError(t, c.writeRepoEntries(entries))
	// Writing the same entry again must not duplicate it.
	require.NoError(t, c.writeRepoEntries(entries))
	rf := readRepoEntries(t, c)
	require.Len(t, rf.Repositories, 1)
}

func TestWriteRepoEntriesNameConflictRejected(t *testing.T) {
	c := newTestCmd(t)
	require.NoError(t, c.writeRepoEntries([]helmRepoEntry{{Name: "repo1", URL: "https://one.example.com"}}))
	err := c.writeRepoEntries([]helmRepoEntry{{Name: "repo1", URL: "https://two.example.com"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different URL")
}

func TestDependencyBuildSkipRefreshOnlyWhenAllWarm(t *testing.T) {
	// With the exec stub capturing args: warm store → --skip-refresh present;
	// cold stores are filled before the binary starts, so both paths skip refresh.
	repoURL := "https://charts.example.com"
	indexData := []byte("apiVersion: v1\nentries: {}\n")

	t.Run("warm store passes skip-refresh", func(t *testing.T) {
		var capturedArgs []string
		c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(cmd *exec.Cmd, _ func(string) string) (string, error) {
			capturedArgs = cmd.Args
			return strings.Join(cmd.Args, " "), nil
		})
		require.NoError(t, err)
		t.Cleanup(func() { c.Close() })

		store := NewIndexStore(t.TempDir(), time.Hour, 1<<30)
		// The store is keyed by repoIdentity (URL + auth fingerprint), the
		// same identity DependencyBuild looks up.
		repo := HelmRepository{Name: "myrepo", Repo: repoURL, Creds: HelmCreds{}}
		require.NoError(t, store.Set(repoIdentity(repo), indexData))
		c.persistentIndexStore = store

		h := &helm{
			cmd:   *c,
			repos: []HelmRepository{repo},
		}
		require.NoError(t, h.DependencyBuild(context.Background()))
		assert.Contains(t, capturedArgs, "--skip-refresh")
	})

	t.Run("cold store is filled before skip-refresh", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(indexData) }))
		t.Cleanup(server.Close)
		var capturedArgs []string
		c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(cmd *exec.Cmd, _ func(string) string) (string, error) {
			capturedArgs = cmd.Args
			return strings.Join(cmd.Args, " "), nil
		})
		require.NoError(t, err)
		t.Cleanup(func() { c.Close() })

		// A cold index is fetched and validated by the provider.
		c.persistentIndexStore = NewIndexStore(t.TempDir(), time.Hour, 1<<30)

		h := &helm{
			cmd:   *c,
			repos: []HelmRepository{{Name: "myrepo", Repo: server.URL, Creds: HelmCreds{}}},
		}
		require.NoError(t, h.DependencyBuild(context.Background()))
		assert.Contains(t, capturedArgs, "--skip-refresh")
	})
}

func TestRepoIdentityAuthScoped(t *testing.T) {
	repoURL := "http://127.0.0.1:18099/"
	r1 := HelmRepository{Name: "r", Repo: repoURL, Creds: HelmCreds{Username: "u1", Password: "p1"}}
	r2 := HelmRepository{Name: "r", Repo: repoURL, Creds: HelmCreds{Username: "u2", Password: "p2"}}
	r1again := HelmRepository{Name: "other-name", Repo: repoURL, Creds: HelmCreds{Username: "u1", Password: "p1"}}

	id1, id2, id1b := repoIdentity(r1), repoIdentity(r2), repoIdentity(r1again)
	// Different credentials on the same URL must not share a cache entry.
	assert.NotEqual(t, id1, id2)
	// Same credentials → stable identity regardless of repo name.
	assert.Equal(t, id1, id1b)
	// No plaintext credential material in the identity.
	assert.NotContains(t, id1, "u1")
	assert.NotContains(t, id1, "p1")
	// Identity is scoped to the URL prefix.
	assert.Contains(t, id1, strings.TrimSuffix(repoURL, "/")+"\x00")
	// Credential-less repos keep a distinct, stable identity.
	anon := repoIdentity(HelmRepository{Name: "r", Repo: repoURL, Creds: HelmCreds{}})
	assert.NotEqual(t, anon, id1)
	assert.Equal(t, anon, repoIdentity(HelmRepository{Name: "r", Repo: repoURL, Creds: HelmCreds{}}))
}

// Regression test for review finding 1: harvesting the SAME bytes back after
func TestWarmBuildDoesNotRestartFreshness(t *testing.T) {
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	repo := HelmRepository{Name: "r", Repo: "https://example.invalid", Creds: HelmCreds{}}
	identity := repoIdentity(repo)
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	old := time.Now().Add(-30 * time.Minute)
	require.NoError(t, os.Chtimes(store.cacheFileName(identity), old, old))
	command, err := newCmdWithVersion(t.TempDir(), false, "", "", func(*exec.Cmd, func(string) string) (string, error) { return "", nil })
	require.NoError(t, err)
	t.Cleanup(command.Close)
	command.persistentIndexStore = store
	app := &helm{cmd: *command, repos: []HelmRepository{repo}}
	require.NoError(t, app.DependencyBuild(t.Context()))
	info, err := os.Stat(store.cacheFileName(identity))
	require.NoError(t, err)
	require.WithinDuration(t, old, info.ModTime(), time.Microsecond)
}

// Finding 3: the repositories.yaml entry written for a repo with a custom CA
// must put the CA path into caFile (helm's trust config), NOT certFile
// (mTLS client cert).
func TestRepoEntryPutsCAPathInCAFile(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, osWriteFile(caPath, []byte("fake-ca"), 0o600))

	repo := HelmRepository{
		Name:  "custom-ca",
		Repo:  "https://example.com/",
		Creds: HelmCreds{CAPath: caPath},
	}
	cmd, err := newCmdWithVersion(t.TempDir(), false, "", "", nil)
	require.NoError(t, err)
	h := &helm{cmd: *cmd}
	entries, err := buildRepoEntries(h, repo)
	require.NoError(t, err)
	require.Len(t, entries, 2, "repo name + helm-manager entries")
	require.Empty(t, entries[0].CertFile, "certFile is for mTLS client certs, never the CA")
	require.Equal(t, caPath, entries[0].CAFile, "CA path must land in caFile so helm trusts the origin")
}
