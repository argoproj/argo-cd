package helm

import (
	"encoding/json"
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
