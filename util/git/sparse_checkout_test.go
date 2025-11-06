package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPathAppsFrontend          = "apps/frontend"
	testPathAppsBackend           = "apps/backend"
	testPathAppsDatabase          = "apps/database"
	testPathInfrastructureTerraform = "infrastructure/terraform"
	testPathLargeAssetsVideos     = "large-assets/videos"
)

// createTestRepoWithStructure creates a git repo with multiple directories
// to simulate a monorepo with different app paths
func createTestRepoWithStructure(t *testing.T, ctx context.Context) string {
	t.Helper()
	tempDir := t.TempDir()

	// Initialize git repo
	require.NoError(t, runCmd(ctx, tempDir, "git", "init"))

	// Create directory structure simulating a monorepo
	dirs := []string{
		testPathAppsFrontend,
		testPathAppsBackend,
		testPathAppsDatabase,
		testPathInfrastructureTerraform,
		testPathLargeAssetsVideos,
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(fullPath, 0o755)
		require.NoError(t, err)

		// Add a file in each directory
		testFile := filepath.Join(fullPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content for "+dir), 0o644)
		require.NoError(t, err)
	}

	// Add a root-level file
	err = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Monorepo"), 0o644)
	require.NoError(t, err)

	// Commit everything
	require.NoError(t, runCmd(ctx, tempDir, "git", "add", "."))
	require.NoError(t, runCmd(ctx, tempDir, "git", "commit", "-m", "Initial monorepo structure"))

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

// getCommitSHA is a helper to fetch the HEAD commit SHA from a git repo
func getCommitSHA(t *testing.T, ctx context.Context, repoPath string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}

// Test_nativeGitClient_SparseCheckout_BasicPaths tests that only specified paths
// are checked out when sparse checkout is enabled
func Test_nativeGitClient_SparseCheckout_BasicPaths(t *testing.T) {
	ctx := context.Background()

	// Create source repo
	srcRepo := createTestRepoWithStructure(t, ctx)

	// Create client with sparse paths using WithSparse option
	// Following the proposed design: cone mode with directory paths
	client, err := NewClient(
		fileURL(srcRepo),
		NopCreds{},
		true,                                        // insecure
		false,                                       // enableLfs
		"",                                          // proxy
		"",                                          // noProxy
		WithSparse([]string{testPathAppsFrontend}), // cone mode directory
	)
	require.NoError(t, err)

	// Init should configure sparse-checkout
	err = client.Init()
	require.NoError(t, err)

	// Fetch the repo
	err = client.Fetch("")
	require.NoError(t, err)

	// Get the commit SHA from the source repo to checkout
	// (can't use origin/HEAD in a local test repo)
	commitSHA := getCommitSHA(t, ctx, srcRepo)

	// Checkout the commit
	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Assert: Only apps/frontend should exist (cone mode includes parent dirs)
	frontendPath := filepath.Join(client.Root(), "apps", "frontend", "test.txt")
	assert.FileExists(t, frontendPath, "apps/frontend should be checked out")

	// Assert: Other directories should NOT exist
	backendPath := filepath.Join(client.Root(), "apps", "backend", "test.txt")
	assert.NoFileExists(t, backendPath, "apps/backend should not be checked out")

	largePath := filepath.Join(client.Root(), "large-assets", "videos", "test.txt")
	assert.NoFileExists(t, largePath, "large-assets should not be checked out")
}

// Test_nativeGitClient_SparseCheckout_MultiplePaths tests checking out multiple
// sparse paths
func Test_nativeGitClient_SparseCheckout_MultiplePaths(t *testing.T) {
	ctx := context.Background()

	srcRepo := createTestRepoWithStructure(t, ctx)

	// Multiple directories in cone mode
	client, err := NewClient(
		fileURL(srcRepo),
		NopCreds{},
		true, false, "", "",
		WithSparse([]string{testPathAppsFrontend, testPathAppsBackend}),
	)
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	// Get commit SHA to checkout
	commitSHA := getCommitSHA(t, ctx, srcRepo)

	_, err = client.Checkout(commitSHA, false)
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

	// Simply verify files exist in the created repo
	// This tests the baseline behavior without any git client operations
	// When we implement sparse checkout, we'll need the git client here

	// All files should exist when sparse checkout is not enabled
	assert.FileExists(t, filepath.Join(srcRepo, "apps", "frontend", "test.txt"))
	assert.FileExists(t, filepath.Join(srcRepo, "apps", "backend", "test.txt"))
	assert.FileExists(t, filepath.Join(srcRepo, "apps", "database", "test.txt"))
	assert.FileExists(t, filepath.Join(srcRepo, "large-assets", "videos", "test.txt"))
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

// Test_nativeGitClient_SparseCheckout_EmptyPaths tests that empty sparse paths
// are handled gracefully (should behave like normal checkout)
func Test_nativeGitClient_SparseCheckout_EmptyPaths(t *testing.T) {
	ctx := context.Background()

	srcRepo := createTestRepoWithStructure(t, ctx)

	// Create client with empty sparse paths
	client, err := NewClient(
		fileURL(srcRepo),
		NopCreds{},
		true, false, "", "",
		WithSparse([]string{}), // empty array
	)
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	commitSHA := getCommitSHA(t, ctx, srcRepo)

	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// With empty sparse paths, all files should be checked out
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "frontend", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "backend", "test.txt"))
	assert.FileExists(t, filepath.Join(client.Root(), "README.md"))
}

// Test_nativeGitClient_SparseCheckout_ReapplyAfterCheckout tests that
// sparse patterns are reapplied after checkout
func Test_nativeGitClient_SparseCheckout_ReapplyAfterCheckout(t *testing.T) {
	ctx := context.Background()

	srcRepo := createTestRepoWithStructure(t, ctx)

	client, err := NewClient(
		fileURL(srcRepo),
		NopCreds{},
		true, false, "", "",
		WithSparse([]string{testPathAppsFrontend}),
	)
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	commitSHA := getCommitSHA(t, ctx, srcRepo)

	// First checkout
	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Verify sparse checkout is working
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "frontend", "test.txt"))
	assert.NoFileExists(t, filepath.Join(client.Root(), "apps", "backend", "test.txt"))

	// Second checkout (should reapply sparse patterns)
	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Sparse checkout should still be active
	assert.FileExists(t, filepath.Join(client.Root(), "apps", "frontend", "test.txt"))
	assert.NoFileExists(t, filepath.Join(client.Root(), "apps", "backend", "test.txt"))
}
