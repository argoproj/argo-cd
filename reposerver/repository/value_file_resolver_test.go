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
			name:          "multiple files",
			rawValueFiles: []string{"values.yaml", "values.yaml"},
			expectedCount: 2,
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

func TestValueFileResolver_resolveLocalValueFile(t *testing.T) {
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

	// Test existing file
	resolvedPath, shouldSkip, err := resolver.resolveLocalValueFile("test.yaml")
	require.NoError(t, err)
	assert.False(t, shouldSkip)
	assert.Contains(t, string(resolvedPath), "test.yaml")

	// Test with URL
	resolvedPath, shouldSkip, err = resolver.resolveLocalValueFile("https://example.com/values.yaml")
	require.NoError(t, err)
	assert.False(t, shouldSkip)
	assert.Equal(t, pathutil.ResolvedFilePath("https://example.com/values.yaml"), resolvedPath)
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

func TestValueFileResolver_resolveReferencedValueFile(t *testing.T) {
	// Test verifies resolution of referenced value files from both Git and OCI repositories
	resolver := NewValueFileResolver(
		"/app",
		"/repo",
		&v1alpha1.Env{},
		[]string{"https"},
		nil,
		utilio.NewRandomizedTempPaths(t.TempDir()),
		utilio.NewRandomizedTempPaths(t.TempDir()),
		false,
	)

	// Test with Git repo
	gitRefTarget := &v1alpha1.RefTarget{
		Repo: v1alpha1.Repository{
			Repo: "https://github.com/test/repo.git",
		},
	}

	_, shouldSkip, _ := resolver.resolveReferencedValueFile("$test/values.yaml", gitRefTarget)
	assert.False(t, shouldSkip) // Referenced files should never be skipped

	// Test with OCI repo
	ociRefTarget := &v1alpha1.RefTarget{
		Repo: v1alpha1.Repository{
			Repo: "oci://registry.example.com/chart",
		},
	}

	_, shouldSkip, _ = resolver.resolveReferencedValueFile("$test/values.yaml", ociRefTarget)
	assert.False(t, shouldSkip)
}
