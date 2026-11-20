package helm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsIndexVersionLookupError(t *testing.T) {
	valid := []string{
		`Error: can't get a valid version for repositories example. Try changing version constraint in Chart.yaml`,
		`Error: chart not found in repo example`,
		`Error: could not find : entry not found`,
		`Error: could not find : no matching version`,
	}
	for _, msg := range valid {
		assert.True(t, isIndexVersionLookupError(errors.New(msg)), msg)
	}
	invalid := []string{
		`Error: looks like "https://charts.example.com/index.yaml" is not a valid chart repository or cannot be reached: connection refused`,
		`Error: failed to fetch oci://registry.example.com/demo : not found`,
		`Error: stat /path/to/dependency-1.0.0.tgz: file:// source is unreachable`,
		`Error: could not download <chart>: failed to get a valid version`,
	}
	for _, msg := range invalid {
		assert.False(t, isIndexVersionLookupError(errors.New(msg)), msg)
	}
}

func TestMissingDependencyVersionFindsConstraintMiss(t *testing.T) {
	dir := t.TempDir()
	writeChartMetadata(t, dir, "Chart.yaml", `apiVersion: v2
name: app
version: 0.1.0
dependencies:
  - name: demo
    version: ">=2.0.0"
    repository: https://charts.example.com
`)

	indexWithV2 := []byte(`apiVersion: v1
entries:
  demo:
    - name: demo
      version: 2.1.0
      urls: ["https://charts.example.com/demo-2.1.0.tgz"]
`)
	indexWithoutV2 := []byte(`apiVersion: v1
entries:
  demo:
    - name: demo
      version: 1.0.0
      urls: ["https://charts.example.com/demo-1.0.0.tgz"]
`)

	assert.False(t, missingDependencyVersion(dir, "https://charts.example.com", nil, indexWithV2), "constraint satisfiable: not a miss")
	assert.True(t, missingDependencyVersion(dir, "https://charts.example.com", nil, indexWithoutV2), "no matching version: a miss")
}

func TestMissingDependencyVersionAliasAndURLMatch(t *testing.T) {
	dir := t.TempDir()
	writeChartMetadata(t, dir, "Chart.yaml", `apiVersion: v2
name: app
version: 0.1.0
dependencies:
  - name: demo
    version: "1.0.0"
    repository: "@example"
`)
	index := []byte(`apiVersion: v1
entries:
  demo:
    - name: demo
      version: 1.0.0
      urls: ["https://charts.example.com/demo-1.0.0.tgz"]
`)

	aliases := map[string]string{"example": "https://charts.example.com"}
	assert.False(t, missingDependencyVersion(dir, "https://charts.example.com", aliases, index), "alias resolves to the same URL: not a miss")
	// A different URL's index is irrelevant to this dependency (skipped, no miss classified).
	assert.False(t, missingDependencyVersion(dir, "https://elsewhere.example.com", nil, index), "different URL: dependency does not target it")
}

func TestMissingDependencyVersionUnsatisfiableConstraint(t *testing.T) {
	dir := t.TempDir()
	writeChartMetadata(t, dir, "Chart.yaml", `apiVersion: v2
name: app
version: 0.1.0
dependencies:
  - name: demo
    version: "not-a-constraint"
    repository: https://charts.example.com
`)
	index := []byte(`apiVersion: v1
entries:
  demo:
    - name: demo
      version: 1.0.0
      urls: ["https://charts.example.com/demo-1.0.0.tgz"]
`)
	assert.False(t, missingDependencyVersion(dir, "https://charts.example.com", nil, index), "invalid constraint: cannot classify")
}

func writeChartMetadata(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
