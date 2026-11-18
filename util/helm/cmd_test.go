package helm

import (
	"context"
	"errors"
	"io"
	"log"
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

func Test_cmd_redactor(t *testing.T) {
	assert.Equal(t, "--foo bar", redactor("--foo bar"))
	assert.Equal(t, "--username ******", redactor("--username bar"))
	assert.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	s, _, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	_, _, err = cmd.template("testdata/chart-does-not-exist", &TemplateOpts{
		KubeVersion: "1.14",
		APIVersions: []string{"foo", "bar"},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "--api-version")
	assert.ErrorContains(t, err, "<api versions removed> ")
}

func TestNewCmd_helmInvalidVersion(t *testing.T) {
	t.Parallel()
	_, err := NewCmd(".", "abcd", "", "")
	log.Println(err)
	assert.EqualError(t, err, "helm version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmd(".", "", "https://proxy:8888", ".argoproj.io")
	require.NoError(t, err)
	assert.Equal(t, "https://proxy:8888", cmd.proxy)
	assert.Equal(t, ".argoproj.io", cmd.noProxy)
}

func TestRegistryLogin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		repo          string
		creds         *HelmCreds
		plainHTTP     bool
		execErr       error
		expectedErr   error
		expectedOut   string
		expectedStdin string
	}{
		{
			name:          "username and password",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:          "username and password with just the hostname",
			repo:          "my.registry.com",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:        "ca file path",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{CAPath: "/path/to/ca"},
			expectedOut: "helm registry login my.registry.com --ca-file /path/to/ca",
		},
		{
			name:        "insecure skip verify",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{InsecureSkipVerify: true},
			expectedOut: "helm registry login my.registry.com --insecure",
		},
		{
			name:        "helm failure",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{},
			execErr:     errors.New("exit status 1"),
			expectedErr: errors.New("failed to login to registry: failed running helm: exit status 1"),
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse registry URL: parse \":///bad-url\": missing protocol scheme"),
		},
		{
			name:          "username & password",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name: "combined flags",
			repo: "my.registry.com:5000/repo",
			creds: &HelmCreds{
				Username:           "u",
				Password:           "p",
				CAPath:             "/ca",
				InsecureSkipVerify: true,
			},
			expectedOut:   "helm registry login my.registry.com:5000 --username u --password-stdin --ca-file /ca --insecure",
			expectedStdin: "p",
		},
		{
			name:          "plain-http",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			plainHTTP:     true,
			expectedOut:   "helm registry login my.registry.com --plain-http --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:        "insecure and plain-http both set",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   true,
			expectedOut: "helm registry login my.registry.com --plain-http --insecure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				var stdin []byte
				if cmd.Stdin != nil {
					var readErr error
					stdin, readErr = io.ReadAll(cmd.Stdin)
					require.NoError(t, readErr)
				}
				assert.Equal(t, tc.expectedStdin, string(stdin))
				if tc.expectedStdin != "" {
					assert.NotContains(t, cmd.Args, tc.expectedStdin)
				}
				if tc.execErr != nil {
					return "", tc.execErr
				}
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.RegistryLogin(t.Context(), tc.repo, tc.creds, tc.plainHTTP)
			assert.Equal(t, tc.expectedOut, out)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPullOCI(t *testing.T) {
	tests := []struct {
		name        string
		creds       HelmCreds
		plainHTTP   bool
		expectedOut string
	}{
		{
			name:        "without flags",
			creds:       HelmCreds{},
			plainHTTP:   false,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest",
		},
		{
			name:        "insecure skip verify",
			creds:       HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   false,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --insecure-skip-tls-verify",
		},
		{
			name:        "plain-http",
			creds:       HelmCreds{},
			plainHTTP:   true,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --plain-http",
		},
		{
			name:        "insecure and plain-http both set",
			creds:       HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   true,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --insecure-skip-tls-verify --plain-http",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.PullOCI("my.registry.com/myrepo", "mychart", "1.0.0", "/tmp/dest", &tc.creds, tc.plainHTTP)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out)
		})
	}
}

func TestDependencyBuild(t *testing.T) {
	tests := []struct {
		name        string
		insecure    bool
		plainHTTP   bool
		expectedOut string
	}{
		{
			name:        "without flags",
			insecure:    false,
			plainHTTP:   false,
			expectedOut: "helm dependency build",
		},
		{
			name:        "with insecure",
			insecure:    true,
			plainHTTP:   false,
			expectedOut: "helm dependency build --insecure-skip-tls-verify",
		},
		{
			name:        "with plain-http",
			insecure:    false,
			plainHTTP:   true,
			expectedOut: "helm dependency build --plain-http",
		},
		{
			name:        "with insecure and plain-http both set",
			insecure:    true,
			plainHTTP:   true,
			expectedOut: "helm dependency build --insecure-skip-tls-verify --plain-http",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.dependencyBuild(t.Context(), tc.insecure, tc.plainHTTP, false)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out)
		})
	}
}

func TestRegistryLogout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		repo        string
		execErr     error
		expectedErr error
		expectedOut string
	}{
		{
			name:        "valid repo",
			repo:        "my.registry.com/repo",
			expectedOut: "helm registry logout my.registry.com",
			expectedErr: nil,
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse registry URL: parse \":///bad-url\": missing protocol scheme"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				if tc.execErr != nil {
					return "", tc.execErr
				}
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.RegistryLogout(tc.repo, nil)
			assert.Equal(t, tc.expectedOut, out)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

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
	cmd := newTestCmd(t)
	h := &helm{cmd: *cmd}
	entries, err := buildRepoEntries(h, repo)
	require.NoError(t, err)
	require.Len(t, entries, 2, "repo name + helm-manager entries")
	require.Empty(t, entries[0].CertFile, "certFile is for mTLS client certs, never the CA")
	require.Equal(t, caPath, entries[0].CAFile, "CA path must land in caFile so helm trusts the origin")
}

func TestHelmPluginsDirectoryMatchesPinnedHelm(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, mode := range []string{"data home", "explicit plugins", "empty plugins", "XDG data", "platform default"} {
		t.Run(mode, func(t *testing.T) {
			for _, key := range []string{"HELM_DATA_HOME", "HELM_PLUGINS", "XDG_DATA_HOME"} {
				t.Setenv(key, "")
				require.NoError(t, os.Unsetenv(key))
			}
			switch mode {
			case "data home":
				t.Setenv("HELM_DATA_HOME", t.TempDir())
			case "explicit plugins":
				t.Setenv("HELM_PLUGINS", t.TempDir())
			case "empty plugins":
				t.Setenv("HELM_PLUGINS", "")
			case "XDG data":
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}
			output, err := exec.CommandContext(t.Context(), "helm", "env", "HELM_PLUGINS").Output()
			require.NoError(t, err)
			require.Equal(t, strings.Trim(strings.TrimSpace(string(output)), "\""), helmPluginsDirectory())
		})
	}
}

func commandEnvironmentValue(values []string, key string) string {
	var result string
	for _, value := range values {
		if after, ok := strings.CutPrefix(value, key+"="); ok {
			result = after
		}
	}
	return result
}

func TestHelmCommandWithoutIndexCacheKeepsPrivateConfig(t *testing.T) {
	t.Setenv("HELM_CONFIG_HOME", t.TempDir())
	cmd := newTestCmd(t)
	var got string
	cmd.runWithRedactor = func(child *exec.Cmd, _ func(string) string) (string, error) {
		got = commandEnvironmentValue(child.Env, "HELM_CONFIG_HOME")
		return "", nil
	}
	_, _, err := cmd.run(t.Context(), "env")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cmd.helmHome, "config"), got, "cache-disabled non-local commands must retain base config isolation")
}

func TestHelmCommandUsesTrustedPluginPaths(t *testing.T) {
	for _, mode := range []string{"empty", "missing home", "relative explicit", "trusted absolute"} {
		t.Run(mode, func(t *testing.T) {
			for _, name := range []string{"HELM_PLUGINS", "HELM_DATA_HOME", "XDG_DATA_HOME"} {
				t.Setenv(name, "")
				require.NoError(t, os.Unsetenv(name))
			}
			trusted := t.TempDir()
			switch mode {
			case "empty":
				t.Setenv("HELM_PLUGINS", "")
			case "missing home":
				t.Setenv("HOME", "")
			case "relative explicit":
				t.Setenv("HELM_PLUGINS", "relative/plugins")
			case "trusted absolute":
				t.Setenv("HELM_PLUGINS", trusted)
			}
			cmd := newTestCmd(t)
			cmd.persistentIndexStore = NewIndexStore(t.TempDir(), time.Minute, 1<<20)
			var got string
			cmd.runWithRedactor = func(child *exec.Cmd, _ func(string) string) (string, error) {
				got = commandEnvironmentValue(child.Env, "HELM_PLUGINS")
				return "", nil
			}
			_, _, err := cmd.run(t.Context(), "env")
			require.NoError(t, err)
			if mode == "trusted absolute" {
				require.Equal(t, trusted, got)
				return
			}
			require.True(t, filepath.IsAbs(got), "empty/missing configuration must not expose plugin paths relative to the checkout: %q", got)
			require.True(t, strings.HasPrefix(got, cmd.helmHome+string(filepath.Separator)), "disabled discovery must use a private directory")
		})
	}
}
