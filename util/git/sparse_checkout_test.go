package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestRepoWithStructure creates a git repo with multiple directories
// to simulate a monorepo with different app paths
func createTestRepoWithStructure(t *testing.T, ctx context.Context) string {
	tempDir := t.TempDir()

	// Initialize git repo
	err := runCmd(ctx, tempDir, "git", "init")
	require.NoError(t, err)

	// Create directory structure simulating a monorepo
	dirs := []string{
		"apps/frontend",
		"apps/backend",
		"apps/database",
		"infrastructure/terraform",
		"large-assets/videos",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(fullPath, 0755)
		require.NoError(t, err)

		// Add a file in each directory
		testFile := filepath.Join(fullPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content for "+dir), 0644)
		require.NoError(t, err)
	}

	// Add a root-level file
	err = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Monorepo"), 0644)
	require.NoError(t, err)

	// Commit everything
	err = runCmd(ctx, tempDir, "git", "add", ".")
	require.NoError(t, err)

	err = runCmd(ctx, tempDir, "git", "commit", "-m", "Initial monorepo structure")
	require.NoError(t, err)

	return tempDir
}

// fileURL converts a local path to a file:// URL, handling Windows paths correctly
func fileURL(path string) string {
	if runtime.GOOS == "windows" {
		// Convert backslashes to forward slashes
		path = strings.ReplaceAll(path, "\\", "/")
		// Windows paths need three slashes: file:///C:/path
		return "file:///" + path
	}
	return "file://" + path
}

// Test_nativeGitClient_SparseCheckout_BasicPaths tests that only specified paths
// are checked out when sparse checkout is enabled
func Test_nativeGitClient_SparseCheckout_BasicPaths(t *testing.T) {
	ctx := context.Background()
	
	// Create source repo
	srcRepo := createTestRepoWithStructure(t, ctx)

	// Create client with sparse paths - only checkout apps/frontend
	client, err := NewClient(fileURL(srcRepo), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	// Initialize with sparse checkout for specific paths
	err = client.Init()
	require.NoError(t, err)

	// TODO: Add method to configure sparse checkout paths
	// err = client.ConfigureSparseCheckout([]string{"apps/frontend/*"})
	// require.NoError(t, err)

	// Fetch the repo
	err = client.Fetch("")
	require.NoError(t, err)

	// Checkout HEAD
	_, err = client.Checkout("HEAD", false)
	require.NoError(t, err)

	// Assert: Only apps/frontend should exist
	frontendPath := filepath.Join(client.Root(), "apps", "frontend", "test.txt")
	assert.FileExists(t, frontendPath, "apps/frontend should be checked out")

	// Assert: Other directories should NOT exist
	backendPath := filepath.Join(client.Root(), "apps", "backend", "test.txt")
	assert.NoFileExists(t, backendPath, "apps/backend should not be checked out")

	largePath := filepath.Join(client.Root(), "large-assets", "videos", "test.txt")
	assert.NoFileExists(t, largePath, "large-assets should not be checked out")

	// Root-level files might or might not exist depending on sparse-checkout config
	// For now, we'll allow them
}

// Test_nativeGitClient_SparseCheckout_MultiplePaths tests checking out multiple
// sparse paths
func Test_nativeGitClient_SparseCheckout_MultiplePaths(t *testing.T) {
	ctx := context.Background()
	
	srcRepo := createTestRepoWithStructure(t, ctx)

	client, err := NewClient(fileURL(srcRepo), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// TODO: Configure sparse checkout for multiple paths
	// err = client.ConfigureSparseCheckout([]string{
	//     "apps/frontend/*",
	//     "apps/backend/*",
	// })
	// require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	_, err = client.Checkout("HEAD", false)
	require.NoError(t, err)

	// Both frontend and backend should exist
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "frontend", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "backend", "test.txt"))

	// But not database or large-assets
	assert.NoFileExists(t, filepath.Join(client.Root(), "apps", "database", "test.txt"))
	assert.NoFileExists(t, filepath.Join(client.Root(), "large-assets", "videos", "test.txt"))
}

// Test_nativeGitClient_SparseCheckout_Disabled tests that when sparse checkout
// is not configured, all files are checked out (current behavior)
func Test_nativeGitClient_SparseCheckout_Disabled(t *testing.T) {
	ctx := context.Background()
	
	srcRepo := createTestRepoWithStructure(t, ctx)

	// Create client without sparse checkout configuration
	client, err := NewClient(fileURL(srcRepo), NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	_, err = client.Checkout("HEAD", false)
	require.NoError(t, err)

	// All files should exist when sparse checkout is not enabled
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "frontend", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "backend", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "database", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "large-assets", "videos", "test.txt"))
}

// Test_nativeGitClient_SparseCheckout_WildcardPatterns tests various git sparse
// checkout pattern syntaxes
func Test_nativeGitClient_SparseCheckout_WildcardPatterns(t *testing.T) {
	t.Skip("TODO: Implement after basic sparse checkout works")
	
	// Test cases for different pattern types:
	// - "apps/frontend" - specific directory
	// - "apps/*" - all subdirectories of apps
	// - "*.yaml" - all yaml files
	// - "!apps/backend" - negative pattern (exclude)
}
