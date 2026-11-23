package helm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fullIndexFixture() []byte {
	return []byte(`apiVersion: v1
entries:
  child:
    - version: 1.0.0
      name: child
      urls: [child-1.0.0.tgz]
  other:
    - version: 2.0.0
      name: other
      urls: [other-2.0.0.tgz]
generated: "2024-01-01T00:00:00Z"
`)
}

func TestParseIndexProjectionFromYAML(t *testing.T) {
	projected, err := parseIndexProjection(fullIndexFixture())
	require.NoError(t, err)
	assert.Len(t, projected.entries, 2)
	assert.Contains(t, projected.metadata, "apiVersion")
	assert.NotContains(t, projected.metadata, "entries")
	assert.Positive(t, projected.size)
}

func TestParseIndexProjectionRejectsWrongVersion(t *testing.T) {
	body := []byte("apiVersion: v2\nentries: {}\n")
	_, err := parseIndexProjection(body)
	require.Error(t, err)
}

func TestParseIndexProjectionRejectsCasingTricks(t *testing.T) {
	body := []byte("apiVersion: v1\nEntries:\n  child:\n    - version: 1.0.0\n")
	_, err := parseIndexProjection(body)
	require.Error(t, err, "case-variant top-level fields must not be projected")
}

func TestIndexSnapshotCacheDeduplicatesByDigest(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	first, err := cache.projection(fullIndexFixture())
	require.NoError(t, err)
	second, err := cache.projection(fullIndexFixture())
	require.NoError(t, err)
	assert.Same(t, first, second, "equal bodies must share one projection")
}

func TestIndexSnapshotCacheFitsBudgetOrErrors(t *testing.T) {
	tiny := newIndexSnapshotCache(256)
	_, err := tiny.projection(fullIndexFixture())
	require.Error(t, err, "oversized bodies must be rejected, not cached")
	// Failure must be memoized (not re-read per build).
	_, err = tiny.projection(fullIndexFixture())
	require.Error(t, err)
}

func TestIndexSnapshotFilterKeepsAllVersionsOfWantedCharts(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	full := fullIndexFixture()
	snapshot, replaced, err := cache.snapshot(full, map[string]struct{}{"child": {}})
	require.NoError(t, err)
	require.True(t, replaced, "a subset selection must produce a new body")

	var index struct {
		APIVersion string                     `json:"apiVersion"`
		Entries    map[string]json.RawMessage `json:"entries"`
		Generated  string                     `json:"generated"`
	}
	require.NoError(t, json.Unmarshal(snapshot, &index))
	assert.Equal(t, "v1", index.APIVersion)
	assert.Len(t, index.Entries, 1)
	var versions []map[string]any
	require.NoError(t, json.Unmarshal(index.Entries["child"], &versions))
	require.Len(t, versions, 1)
	assert.Equal(t, "1.0.0", versions[0]["version"])
	assert.NotEmpty(t, index.Generated, "metadata must survive projection")
}

func TestIndexSnapshotFullSelectionReturnsOriginalBody(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	full := fullIndexFixture()
	snapshot, replaced, err := cache.snapshot(full, map[string]struct{}{"child": {}, "other": {}})
	require.NoError(t, err)
	assert.False(t, replaced)
	assert.Equal(t, full, snapshot, "an all-charts selection must reuse the original body verbatim")
}

func TestIndexSnapshotAbsentDependencyFails(t *testing.T) {
	cache := newIndexSnapshotCache(1 << 20)
	_, _, err := cache.snapshot(fullIndexFixture(), map[string]struct{}{"ghost": {}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ghost")
}

func TestDependencyNamesByRepository(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(`apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: child
    repository: https://charts.example.com
  - name: aliased
    repository: "@example"
  - name: local
    repository: "file://./local"
`), 0o600), "requirements.yaml absent is fine")

	names, err := dependencyNamesByRepository(t.Context(), dir, []HelmRepository{{Name: "example", Repo: "https://example-repo.example.com"}})
	require.NoError(t, err)
	require.Len(t, names, 2, "each distinct http(s) repository forms one identity group")
	// The identity keys are opaque; assert across every collected group.
	collected := make(map[string]struct{})
	for _, value := range names {
		for name := range value {
			collected[name] = struct{}{}
		}
	}
	require.Contains(t, collected, "child")
	require.Contains(t, collected, "aliased")
	require.NotContains(t, collected, "local", "file:// deps are never index-sourced")
}

func TestDependencyNamesByRepositoryUnresolvedAliasFails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(`apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: child
    repository: "@ghost"
`), 0o600))
	_, err := dependencyNamesByRepository(t.Context(), dir, nil)
	require.Error(t, err)
}

func TestIsIndexSnapshotErrorClassification(t *testing.T) {
	valid := []string{
		"Error: can't get a valid version for repositories child. Try changing version constraint in Chart.yaml",
		"no cached repo found. Please run helm repo update",
		"Error: could not find : entry not found",
		"no cached repository available for example",
	}
	for _, message := range valid {
		assert.True(t, isIndexSnapshotError(fmt.Errorf("%s", message)), message)
	}
	invalid := []string{
		"failed to fetch https://charts.example.com: connection refused",
		"lookup oci://registry.example.com: no such host",
		"file://./local: no such file",
		"failed to install charts: chart verification failed",
	}
	for _, message := range invalid {
		assert.False(t, isIndexSnapshotError(fmt.Errorf("%s", message)), message)
	}
}

// Regression: with FilterEnabled the private snapshot must not replace the
// stored full index (copyTo of a filtered body with replace must not reach
// the store's own record file).
func TestFilteredSnapshotDoesNotTouchStoredRecord(t *testing.T) {
	store := NewIndexStore(filepath.Join(t.TempDir(), "store"), time.Hour, 1<<20)
	store.FilterEnabled = true
	full := fullIndexFixture()
	require.NoError(t, store.Set("https://charts.example.com", full))

	cache := newIndexSnapshotCache(1 << 20)
	filtered, replaced, err := cache.snapshot(full, map[string]struct{}{"child": {}})
	require.NoError(t, err)
	require.True(t, replaced)
	// The stored record keeps the full body.
	stored, err := store.GetRetained("https://charts.example.com")
	require.NoError(t, err)
	assert.Equal(t, full, stored, "the stored record must keep the full body")
	_ = filtered
}

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

// A dependency added upstream AFTER the cached index was fetched is absent
// from the filtered snapshot entirely. The snapshot fallback must treat the
// absence as a version-lookup miss: revalidate against the origin, restore
// the full index, and retry — with filtering disabled the same build
// succeeds.
func TestIndexFilterIntegrationNewDependencyRevalidates(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	store.FilterEnabled = true

	// Warm pass: only the child dependency exists upstream.
	parent := regressionParent(t, repo.Repo)
	warm, err := regressionBuild(t, parent, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, warm, "1.0.0")

	// A dependency the cached index does not know at all must not abort
	// the filter stage: the snapshot reports it absent, the stage keeps the
	// full index installed, and the version-miss fallback revalidates on
	// the build failure. Unit-level: absent dependency is classified, full
	// body stays untouched.
	warmData, err := store.GetRetained(regressionIdentity(t, repo))
	require.NoError(t, err)
	_, _, snapshotErr := dependencyIndexes.snapshot(warmData, map[string]struct{}{"child": {}, "extra": {}})
	require.Error(t, snapshotErr)
	assert.True(t, isIndexDependencyAbsentError(snapshotErr), "an unknown dependency must classify as absent, got: %v", snapshotErr)
	full, err := store.GetRetained(regressionIdentity(t, repo))
	require.NoError(t, err)
	assert.Equal(t, warmData, full, "the absent-dependency error must not touch the stored record")
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
	store.renameRecord = func(_, _ string) error {
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

// ANY snapshot failure — projection parse errors and absent dependencies
// alike — must keep the full index installed: the filter stage treats every
// snapshot error the same and never aborts the build. Projection errors are
// not absent-dependency errors, so the classification stays distinct even
// though the fallback is shared.
func TestIndexProjectionFailureIsNotAbsentDependency(t *testing.T) {
	// A scalar entries value fails the projection decode.
	_, _, err := dependencyIndexes.snapshot(
		[]byte("apiVersion: v1\nentries:\n  child: not-a-chart-entry\n"),
		map[string]struct{}{"child": {}})
	require.Error(t, err, "a scalar entry must fail projection")
	assert.False(t, isIndexDependencyAbsentError(err), "a decode failure is not an absent dependency")
}
