package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	pathutil "github.com/argoproj/argo-cd/v3/util/io/path"
)

func TestValueFileResolver_ResolveValueFiles(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app")
	repoRoot := tmpDir

	// Create test files
	require.NoError(t, os.MkdirAll(appPath, 0o755))
	testValueFile := filepath.Join(appPath, "values.yaml")
	require.NoError(t, os.WriteFile(testValueFile, []byte("test: value"), 0o644))

	tests := []struct {
		name                    string
		rawValueFiles           []string
		ignoreMissingValueFiles bool
		expectError             bool
		expectedCount           int
	}{
		{
			name:          "resolve local file",
			rawValueFiles: []string{"values.yaml"},
			expectedCount: 1,
		},
		{
			name:                    "ignore missing file",
			rawValueFiles:           []string{"missing.yaml"},
			ignoreMissingValueFiles: true,
			expectedCount:           0,
		},
		{
			name:          "duplicate files are de-duplicated",
			rawValueFiles: []string{"values.yaml", "values.yaml"},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewValueFileResolver(
				appPath,
				repoRoot,
				&v1alpha1.Env{},
				[]string{"https", "http"},
				nil, // no ref sources for this test
				utilio.NewRandomizedTempPaths(t.TempDir()),
				utilio.NewRandomizedTempPaths(t.TempDir()),
				tt.ignoreMissingValueFiles,
			)

			result, err := resolver.ResolveValueFiles(tt.rawValueFiles)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
		})
	}
}

func TestValueFileResolver_resolveRawPath_local(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app")
	repoRoot := tmpDir

	require.NoError(t, os.MkdirAll(appPath, 0o755))
	testFile := filepath.Join(appPath, "test.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))

	resolver := NewValueFileResolver(
		appPath,
		repoRoot,
		&v1alpha1.Env{},
		[]string{"https", "http"},
		nil,
		utilio.NewRandomizedTempPaths(t.TempDir()),
		utilio.NewRandomizedTempPaths(t.TempDir()),
		false,
	)

	// Test existing local file
	resolved, err := resolver.resolveRawPath("test.yaml")
	require.NoError(t, err)
	assert.False(t, resolved.IsRemote)
	assert.Equal(t, repoRoot, resolved.EffectiveRoot)
	assert.Contains(t, string(resolved.Path), "test.yaml")

	// Test with URL
	resolved, err = resolver.resolveRawPath("https://example.com/values.yaml")
	require.NoError(t, err)
	assert.True(t, resolved.IsRemote)
	assert.Equal(t, pathutil.ResolvedFilePath("https://example.com/values.yaml"), resolved.Path)
}

func TestValueFileResolver_checkFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.yaml")
	require.NoError(t, os.WriteFile(existingFile, []byte("test"), 0o644))

	tests := []struct {
		name                    string
		path                    pathutil.ResolvedFilePath
		ignoreMissingValueFiles bool
		expectedSkip            bool
	}{
		{
			name:         "existing file",
			path:         pathutil.ResolvedFilePath(existingFile),
			expectedSkip: false,
		},
		{
			name:                    "missing file with ignore",
			path:                    pathutil.ResolvedFilePath(filepath.Join(tmpDir, "missing.yaml")),
			ignoreMissingValueFiles: true,
			expectedSkip:            true,
		},
		{
			name:                    "missing file without ignore",
			path:                    pathutil.ResolvedFilePath(filepath.Join(tmpDir, "missing.yaml")),
			ignoreMissingValueFiles: false,
			expectedSkip:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &ValueFileResolver{
				ignoreMissingValueFiles: tt.ignoreMissingValueFiles,
			}

			shouldSkip := resolver.checkFileExists(tt.path)
			assert.Equal(t, tt.expectedSkip, shouldSkip)
		})
	}
}

func TestValueFileResolver_resolveRawPath_referenced(t *testing.T) {
	// Test verifies resolution of referenced value files from both Git and OCI repositories.
	// Neither ref source is present in the temp paths, so resolution returns an error rather
	// than panicking - the point is that the ref branch is exercised for both URL schemes.
	resolver := NewValueFileResolver(
		"/app",
		"/repo",
		&v1alpha1.Env{},
		[]string{"https"},
		map[string]*v1alpha1.RefTarget{
			"$git": {Repo: v1alpha1.Repository{Repo: "https://github.com/test/repo.git"}},
			"$oci": {Repo: v1alpha1.Repository{Repo: "oci://registry.example.com/chart"}},
		},
		utilio.NewRandomizedTempPaths(t.TempDir()),
		utilio.NewRandomizedTempPaths(t.TempDir()),
		false,
	)

	// Git ref source - repo not registered in temp paths, so it errors out.
	_, err := resolver.resolveRawPath("$git/values.yaml")
	require.Error(t, err)

	// OCI ref source - repo not registered in temp paths, so it errors out.
	_, err = resolver.resolveRawPath("$oci/values.yaml")
	require.Error(t, err)
}
