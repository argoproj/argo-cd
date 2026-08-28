//go:build !windows

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Test_nativeGitClient_CheckoutRecoversLockFromKilledGit is the end-to-end form of
// the stale-lock recovery: rather than writing a lock file by hand, it strands one
// with a real killed git process.
//
// git holds .git/HEAD.lock across the reference-transaction hook's "prepared"
// state, so a hook that blocks there keeps the ref lock open for as long as the
// test needs. SIGKILLing the process group at that point is what production does
// when the exec timeout fires or a gRPC caller cancels mid-checkout, and it leaves
// exactly the artifact users report: a plain lock file nothing reclaims, wedging
// every later operation on that cached repo.
//
// Unix-only: it relies on process groups and signals.
func Test_nativeGitClient_CheckoutRecoversLockFromKilledGit(t *testing.T) {
	ctx := t.Context()

	originDir, err := _createEmptyGitRepo(ctx)
	require.NoError(t, err)
	require.NoError(t, runCmd(ctx, originDir, "git", "commit", "-m", "Second commit", "--allow-empty"))

	client, err := NewClient("file://"+originDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)
	require.NoError(t, client.Init())
	require.NoError(t, client.Fetch(ctx, "", 0))

	tipSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	_, err = client.Checkout(ctx, tipSHA, false, true)
	require.NoError(t, err)

	root := client.Root()
	lockPath := filepath.Join(root, ".git", "HEAD.lock")

	hookPath := filepath.Join(root, ".git", "hooks", "reference-transaction")
	require.NoError(t, os.MkdirAll(filepath.Dir(hookPath), 0o755))
	require.NoError(t, os.WriteFile(hookPath,
		[]byte("#!/bin/sh\nif [ \"$1\" = \"prepared\" ]; then sleep 60; fi\n"), 0o755))

	// Moving HEAD to a different commit is what takes HEAD.lock; re-checking out the
	// current commit is a no-op and locks nothing.
	prevOut, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD~1").Output()
	require.NoError(t, err)

	cmd := exec.CommandContext(ctx, "git", "checkout", "--force", strings.TrimSpace(string(prevOut)))
	cmd.Dir = root
	// A global core.hooksPath would silently override the repository's own hooks and
	// skip the hook this test depends on.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())

	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, statErr := os.Lstat(lockPath); statErr == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			t.Fatal("git never took HEAD.lock")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// kill git and the hook together, mid-transaction; no cleanup of any kind runs
	require.NoError(t, syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL))
	_ = cmd.Wait()
	require.NoError(t, os.Remove(hookPath))

	// the killed process left a real lock, and the cached repo is now wedged
	require.FileExists(t, lockPath)
	require.Error(t, runCmd(ctx, root, "git", "checkout", "--force", tipSHA))

	out, err := client.Checkout(ctx, tipSHA, false, true)
	require.NoError(t, err, "repo stayed wedged after a killed git process; output: %s", out)
	require.NoFileExists(t, lockPath)
}
