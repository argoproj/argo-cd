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

	// Create a bare repository to act as the "remote"
	remoteDir := t.TempDir()
	remotePath := filepath.Join(remoteDir, "remote.git")
	err := os.MkdirAll(remotePath, 0755)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = runGitCmd(ctx, remotePath, "init", "--bare")
	require.NoError(t, err)

	// Create a local repository with some commits
	localDir := t.TempDir()
	localPath := filepath.Join(localDir, "local")
	err = os.MkdirAll(localPath, 0755)
	require.NoError(t, err)

	// Initialize and create initial commits
	_, err = runGitCmd(ctx, localPath, "init")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "remote", "add", "origin", remotePath)
	require.NoError(t, err)

	// Create 3 branches with commits (simulating different hydration targets)
	branches := []string{"env/dev", "env/staging", "env/prod"}
	commitSHAs := make([]string, 3)

	for i, branch := range branches {
		testFile := filepath.Join(localPath, fmt.Sprintf("file-%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)

		_, err = runGitCmd(ctx, localPath, "add", ".")
		require.NoError(t, err)
		_, err = runGitCmd(ctx, localPath, "commit", "-m", fmt.Sprintf("commit %s", branch))
		require.NoError(t, err)

		sha, err := runGitCmd(ctx, localPath, "rev-parse", "HEAD")
		require.NoError(t, err)
		commitSHAs[i] = sha

		_, err = runGitCmd(ctx, localPath, "branch", branch)
		require.NoError(t, err)
		_, err = runGitCmd(ctx, localPath, "push", "origin", branch)
		require.NoError(t, err)
	}

	// Create separate clones for concurrent operations
	gitClients := make([]git.Client, 3)

	for i := 0; i < 3; i++ {
		workDir := filepath.Join(localDir, fmt.Sprintf("work-%d", i))
		err = os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

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

		gitClients[i] = client
	}

	// Add notes concurrently with slight stagger
	var wg sync.WaitGroup
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			time.Sleep(time.Duration(idx*50) * time.Millisecond)
			errors[idx] = AddNote(gitClients[idx], fmt.Sprintf("dry-sha-%d", idx), commitSHAs[idx])
		}(i)
	}
	wg.Wait()

	// Verify all notes persisted
	verifyClient, err := git.NewClientExt(remotePath, t.TempDir(), &git.NopCreds{}, false, false, "", "")
	require.NoError(t, err)
	err = verifyClient.Init()
	require.NoError(t, err)
	err = verifyClient.Fetch("", 0)
	require.NoError(t, err)

	for i, commitSHA := range commitSHAs {
		note, err := verifyClient.GetCommitNote(commitSHA, NoteNamespace)
		assert.NoError(t, err, "Note should exist for commit %d", i)
		if err == nil {
			assert.Contains(t, note, fmt.Sprintf("dry-sha-%d", i))
		}
	}
}

// TestAddNoteConcurrentSimultaneous tests that when multiple AddNote operations run
// simultaneously (without delays), all notes persist correctly.
// Each operation gets its own git clone, simulating multiple concurrent hydration requests.
func TestAddNoteConcurrentSimultaneous(t *testing.T) {
	t.Parallel()

	remoteDir := t.TempDir()
	remotePath := filepath.Join(remoteDir, "remote.git")
	err := os.MkdirAll(remotePath, 0755)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = runGitCmd(ctx, remotePath, "init", "--bare")
	require.NoError(t, err)

	localDir := t.TempDir()
	localPath := filepath.Join(localDir, "local")
	err = os.MkdirAll(localPath, 0755)
	require.NoError(t, err)

	_, err = runGitCmd(ctx, localPath, "init")
	require.NoError(t, err)
	_, err = runGitCmd(ctx, localPath, "config", "user.name", "Test User")
	require.NoError(t, err)
	_, err = runGitCmd(ctx, localPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	_, err = runGitCmd(ctx, localPath, "remote", "add", "origin", remotePath)
	require.NoError(t, err)

	// Create 3 branches with commits (simulating different hydration targets)
	branches := []string{"env/dev", "env/staging", "env/prod"}
	commitSHAs := make([]string, 3)

	for i, branch := range branches {
		testFile := filepath.Join(localPath, fmt.Sprintf("file-%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)

		_, err = runGitCmd(ctx, localPath, "add", ".")
		require.NoError(t, err)
		_, err = runGitCmd(ctx, localPath, "commit", "-m", fmt.Sprintf("commit %s", branch))
		require.NoError(t, err)

		sha, err := runGitCmd(ctx, localPath, "rev-parse", "HEAD")
		require.NoError(t, err)
		commitSHAs[i] = sha

		_, err = runGitCmd(ctx, localPath, "branch", branch)
		require.NoError(t, err)
		_, err = runGitCmd(ctx, localPath, "push", "origin", branch)
		require.NoError(t, err)
	}

	// Create separate clones for concurrent operations
	gitClients := make([]git.Client, 3)

	for i := 0; i < 3; i++ {
		workDir := filepath.Join(localDir, fmt.Sprintf("work-%d", i))
		err = os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

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

		gitClients[i] = client
	}

	// Add notes concurrently without delays
	var wg sync.WaitGroup
	startChan := make(chan struct{})

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startChan
			AddNote(gitClients[idx], fmt.Sprintf("dry-sha-%d", idx), commitSHAs[idx])
		}(i)
	}

	close(startChan)
	wg.Wait()

	// Verify all notes persisted
	verifyClient, err := git.NewClientExt(remotePath, t.TempDir(), &git.NopCreds{}, false, false, "", "")
	require.NoError(t, err)
	err = verifyClient.Init()
	require.NoError(t, err)
	err = verifyClient.Fetch("", 0)
	require.NoError(t, err)

	for i, commitSHA := range commitSHAs {
		note, err := verifyClient.GetCommitNote(commitSHA, NoteNamespace)
		assert.NoError(t, err, "Note should exist for commit %d", i)
		if err == nil {
			assert.Contains(t, note, fmt.Sprintf("dry-sha-%d", i))
		}
	}
}

// runGitCmd is a helper function to run git commands
func runGitCmd(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
