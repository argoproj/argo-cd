package security

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceToCurrentRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup: Create files with specific content so we can verify we opened the right one.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("root-file"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "charts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "charts", "values.yaml"), []byte("subdir-file"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c", "d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a", "b", "c", "d", "values.yaml"), []byte("deep-file"), 0o644))

	// Helper to verify we received a valid file handle pointing to the expected data.
	verifyFileContent := func(t *testing.T, f *os.File, expectedContent string) {
		t.Helper()
		defer f.Close()
		content, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, expectedContent, string(content))
	}

	t.Run("file directly in root", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "values.yaml")
		fileHandle, err := EnforceToCurrentRoot(tmpDir, testFile)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		verifyFileContent(t, fileHandle, "root-file")
	})

	t.Run("file in subdirectory", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "charts", "values.yaml")
		fileHandle, err := EnforceToCurrentRoot(tmpDir, testFile)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		verifyFileContent(t, fileHandle, "subdir-file")
	})

	t.Run("file outside current working directory", func(t *testing.T) {
		outsidePath := filepath.Join(filepath.Dir(tmpDir), "values.yaml")
		_, err := EnforceToCurrentRoot(tmpDir, outsidePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "should be on or under current directory")
	})

	t.Run("file escapes using parent directory notation", func(t *testing.T) {
		escapePath := filepath.Join(tmpDir, "..", "differentapp", "values.yaml")
		_, err := EnforceToCurrentRoot(tmpDir, escapePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "should be on or under current directory")
	})

	t.Run("goes back and forth but remains within root", func(t *testing.T) {
		complexPath := filepath.Join(tmpDir, "charts", "..", "values.yaml")
		fileHandle, err := EnforceToCurrentRoot(tmpDir, complexPath)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		verifyFileContent(t, fileHandle, "root-file")
	})

	t.Run("path equals root directory", func(t *testing.T) {
		// Opening the directory itself should succeed
		fileHandle, err := EnforceToCurrentRoot(tmpDir, tmpDir)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		defer fileHandle.Close()

		info, err := fileHandle.Stat()
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("complex path with multiple . and ..", func(t *testing.T) {
		complexPath := filepath.Join(tmpDir, ".", "charts", "..", "values.yaml")
		fileHandle, err := EnforceToCurrentRoot(tmpDir, complexPath)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		verifyFileContent(t, fileHandle, "root-file")
	})

	t.Run("root with trailing slash vs without", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "values.yaml")

		// 1. With Slash.
		rootWithSlash := tmpDir + string(filepath.Separator)
		f1, err1 := EnforceToCurrentRoot(rootWithSlash, testFile)
		require.NoError(t, err1)
		verifyFileContent(t, f1, "root-file")

		// 2. Without Slash.
		f2, err2 := EnforceToCurrentRoot(tmpDir, testFile)
		require.NoError(t, err2)
		verifyFileContent(t, f2, "root-file")
	})

	t.Run("attempt to escape with multiple parent references", func(t *testing.T) {
		escapePath := filepath.Join(tmpDir, "..", "..", "..", "etc", "passwd")
		_, err := EnforceToCurrentRoot(tmpDir, escapePath)
		require.Error(t, err)
	})

	t.Run("deep nested subdirectory", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "a", "b", "c", "d", "values.yaml")
		fileHandle, err := EnforceToCurrentRoot(tmpDir, testFile)
		require.NoError(t, err)
		require.NotNil(t, fileHandle)
		verifyFileContent(t, fileHandle, "deep-file")
	})
}

func TestEnforceToCurrentRootEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("test"), 0o644))

	t.Run("empty paths", func(t *testing.T) {
		_, err := EnforceToCurrentRoot("", "/some/path")
		require.Error(t, err)
	})

	t.Run("relative root path", func(t *testing.T) {
		// EnforceToCurrentRoot expects absolute paths for the root (usually)
		// os.OpenRoot might work with relative paths, but filepath.Rel logic depends on clean comparisons.
		_, err := EnforceToCurrentRoot("relative/path", "/absolute/path")
		require.Error(t, err)
	})

	t.Run("path with double slashes", func(t *testing.T) {
		doubleSlashPath := tmpDir + string(filepath.Separator) + string(filepath.Separator) + "values.yaml"
		f, err := EnforceToCurrentRoot(tmpDir, doubleSlashPath)
		require.NoError(t, err)
		defer f.Close()
	})

	t.Run("non-existent file in valid directory", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "does-not-exist.yaml")
		_, err := EnforceToCurrentRoot(tmpDir, nonExistentPath)
		// This should fail because root.Open() checks if file exists.
		require.Error(t, err)
	})
}

func TestEnforceToCurrentRootWindowsPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific tests on non-Windows platform")
	}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "values.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))

	t.Run("windows path in root", func(t *testing.T) {
		f, err := EnforceToCurrentRoot(tmpDir, testFile)
		require.NoError(t, err)
		defer f.Close()
	})

	t.Run("windows path escape attempt", func(t *testing.T) {
		escapePath := filepath.Join(tmpDir, "..", "..", "Windows", "System32", "config")
		_, err := EnforceToCurrentRoot(tmpDir, escapePath)
		require.Error(t, err)
	})
}
