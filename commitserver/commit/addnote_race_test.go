package commit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/git"
)

// TestAddNoteConcurrentStaggered tests that when multiple AddNote operations run
// with slightly staggered timing, all notes persist correctly.
// Each operation gets its own git clone, simulating multiple concurrent hydration requests.
func TestAddNoteConcurrentStaggered(t *testing.T) {
	t.Parallel()

	remotePath, localPath := setupRepoWithRemote(t)

	// Create 3 branches with commits (simulating different hydration targets)
	branches := []string{"env/dev", "env/staging", "env/prod"}
	commitSHAs := make([]string, 3)

	for i, branch := range branches {
		commitSHAs[i] = commitAndPushBranch(t, localPath, branch)
	}

	// Create separate clones for concurrent operations
	cloneClients := make([]git.Client, 3)
	for i := range 3 {
		cloneClients[i] = getClientForClone(t, remotePath)
	}

	// Add notes concurrently with slight stagger
	var wg sync.WaitGroup
	errors := make([]error, 3)

	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			time.Sleep(time.Duration(idx*50) * time.Millisecond)
			errors[idx] = AddNote(cloneClients[idx], fmt.Sprintf("dry-sha-%d", idx), commitSHAs[idx])
		}(i)
	}
	wg.Wait()

	// Verify all notes persisted
	verifyClient := getClientForClone(t, remotePath)

	for i, commitSHA := range commitSHAs {
		note, err := verifyClient.GetCommitNote(commitSHA, NoteNamespace)
		require.NoError(t, err, "Note should exist for commit %d", i)
		assert.Contains(t, note, fmt.Sprintf("dry-sha-%d", i))
	}
}

// TestAddNoteConcurrentSimultaneous tests that when multiple AddNote operations run
// simultaneously (without delays), all notes persist correctly.
// Each operation gets its own git clone, simulating multiple concurrent hydration requests.
func TestAddNoteConcurrentSimultaneous(t *testing.T) {
	t.Parallel()

	remotePath, localPath := setupRepoWithRemote(t)

	// Create 3 branches with commits (simulating different hydration targets)
	branches := []string{"env/dev", "env/staging", "env/prod"}
	commitSHAs := make([]string, 3)

	for i, branch := range branches {
		commitSHAs[i] = commitAndPushBranch(t, localPath, branch)
	}

	// Create separate clones for concurrent operations
	cloneClients := make([]git.Client, 3)
	for i := range 3 {
		cloneClients[i] = getClientForClone(t, remotePath)
	}

	// Add notes concurrently without delays
	var wg sync.WaitGroup
	startChan := make(chan struct{})

	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startChan
			_ = AddNote(cloneClients[idx], fmt.Sprintf("dry-sha-%d", idx), commitSHAs[idx])
		}(i)
	}

	close(startChan)
	wg.Wait()

	// Verify all notes persisted
	verifyClient := getClientForClone(t, remotePath)

	for i, commitSHA := range commitSHAs {
		note, err := verifyClient.GetCommitNote(commitSHA, NoteNamespace)
		require.NoError(t, err, "Note should exist for commit %d", i)
		assert.Contains(t, note, fmt.Sprintf("dry-sha-%d", i))
	}
}

// setupRepoWithRemote creates a bare remote repo and a local repo configured to push to it.
// Returns the remote path and local path.
func setupRepoWithRemote(t *testing.T) (remotePath, localPath string) {
	t.Helper()
	ctx := t.Context()

	// Create bare remote repository
	remoteDir := t.TempDir()
	remotePath = filepath.Join(remoteDir, "remote.git")
	err := os.MkdirAll(remotePath, 0o755)
	require.NoError(t, err)

	_, err = runGitCmd(ctx, remotePath, "init", "--bare")
	require.NoError(t, err)

	// Create local repository
	localDir := t.TempDir()
	localPath = filepath.Join(localDir, "local")
	err = os.MkdirAll(localPath, 0o755)
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "init")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "remote", "add", "origin", remotePath)
	require.NoError(t, err)

	return remotePath, localPath
}

// commitAndPushBranch writes a file, commits it, creates a branch, and pushes to remote.
// Returns the commit SHA.
func commitAndPushBranch(t *testing.T, localPath, branch string) string {
	t.Helper()
	ctx := t.Context()

	testFile := filepath.Join(localPath, "test.txt")
	err := os.WriteFile(testFile, []byte("content for "+branch), 0o644)
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "add", ".")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "commit", "-m", "commit "+branch)
	require.NoError(t, err)

	sha, err := runGitCmd(ctx, localPath, "rev-parse", "HEAD")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "branch", branch)
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "push", "origin", branch)
	require.NoError(t, err)

	return sha
}

// getClientForClone creates a git client with a fresh clone of the remote repo.
func getClientForClone(t *testing.T, remotePath string) git.Client {
	t.Helper()
	ctx := t.Context()

	workDir := t.TempDir()

	client, err := git.NewClientExt(remotePath, workDir, &git.NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	_, err = runGitCmd(ctx, workDir, "config", "user.name", "Test User")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, workDir, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	err = client.Fetch("", 0)
	require.NoError(t, err)

	return client
}

// runGitCmd is a helper function to run git commands
func runGitCmd(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
