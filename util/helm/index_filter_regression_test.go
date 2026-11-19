package helm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The filter groups dependency names under the identity the store lookup
// uses. dependencyNamesByRepository is credential-blind and keys its map by
// unscoped identity, so the lookup must translate — comparing a scoped
// identity directly against the unscoped map silently disables filtering.
func TestDependencyFilterIdentityMatchesLookupKey(t *testing.T) {
	repos := []HelmRepository{
		{Name: "primary", Repo: "https://charts.example.com"},
		{Name: "aliased", Repo: "https://example-repo.example.com"},
	}
	names, err := dependencyNamesByRepository(t.Context(), regressionParent(t, "https://charts.example.com"), repos)
	require.NoError(t, err)
	require.NotEmpty(t, names)

	unscopedPrimary, err := scopedIndexIdentityContext(t.Context(), "https://charts.example.com", "", "", "")
	require.NoError(t, err)
	scopedPrimary, err := scopedIndexIdentityContext(t.Context(), "https://charts.example.com", "argon2id-v1:aa", "", "")
	require.NoError(t, err)
	assert.NotEqual(t, unscopedPrimary, scopedPrimary)
	assert.Contains(t, names, unscopedPrimary, "filter map must be keyed by the unscoped identity")
	wanted := names[unscopedPrimary]
	assert.Contains(t, wanted, "child", "the dependency chart name must be grouped under its repository identity")
}

// With filtering enabled, a version missing from the stale filtered snapshot
// (a newer release was published upstream) must revalidate against the
// origin before the full-index fallback retries, otherwise the retry would
// repeat the same stale miss and never observe the new version.
func TestIndexFilterIntegrationVersionMissRevalidates(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	store.FilterEnabled = true

	// Pin the dependency to exactly 2.0.0: a range constraint would be
	// satisfied by the stale 1.0.0 snapshot and never exercise the miss.
	parent := regressionParent(t, repo.Repo)
	pinnedChart := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: '2.0.0'\n    repository: %q\n", repo.Repo)
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), []byte(pinnedChart), 0o600))

	warm, err := regressionBuild(t, parent, repo, store, false)
	require.Error(t, err, "the warm pass targets 2.0.0 while the origin still serves 1.0.0; it must fail with the version lookup error")
	assert.Contains(t, err.Error(), "version", "warm failure should be a version lookup miss, got: %v", err)
	_ = warm

	fixture.publish("2.0.0")
	fresh, err := regressionBuild(t, parent, repo, store, false)
	require.NoError(t, err, "a filtered snapshot miss on a NEW version must revalidate, not fail")
	regressionRender(t, fresh, "2.0.0")
}

// Publication must invalidate the previous record BEFORE preparing the
// replacement body: a temp-file failure mid-publication must not leave the
// current build holding fresh bytes while the old record stays installed —
// the next ordinary build would silently roll back to the stale index.
func TestPublishRecordInvalidatesBeforeTempFile(t *testing.T) {
	store, dir := newTestIndexStore(t, time.Minute, 1<<20)
	fixture := &regressionChartRepo{version: "1.0.0", packages: map[string][]byte{"1.0.0": regressionPackage(t, "child", "1.0.0")}}
	body := fixture.index()
	require.NoError(t, store.Set("https://rollback.example.com", body))
	first, err := store.Get("https://rollback.example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, first)

	// Force CreateTemp to fail by injecting a renameRecord that simulates the
	// temp-file stage failing: publication returns an error after the
	// invalidation step has run. The injected failure models ANY failure
	// between invalidation and rename (temp file create/write/close).
	require.NoError(t, os.Chmod(dir, 0o700))
	store.renameRecord = func(from, to string) error {
		return errors.New("simulated temp-file stage failure")
	}

	err = store.Set("https://rollback.example.com", fixture.index())
	require.Error(t, err, "a failing publication stage must surface")
	assert.Contains(t, err.Error(), "simulated")

	_, getErr := store.Get("https://rollback.example.com")
	require.Error(t, getErr, "a failed publication must not leave the old record servable")
	assert.Contains(t, getErr.Error(), "unconfirmed",
		"the pre-publication invalidation must mark the record, got: %v", getErr)
}
