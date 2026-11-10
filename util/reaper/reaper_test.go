package reaper

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartTrackedAndUntrack(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "true")
	token, err := StartTracked(cmd)
	require.NoError(t, err)
	pid := cmd.Process.Pid
	assert.True(t, isTracked(pid))
	require.NoError(t, cmd.Wait())
	Untrack(pid, token)
	assert.False(t, isTracked(pid))
}

// TestUntrack_StaleTokenDoesNotRemoveNewerEntry simulates the kernel reusing
// a pid for a newer tracked command before the older invocation's deferred
// Untrack runs: the stale Untrack must be a no-op, or the newer command's
// child would become visible to the reaper and its cmd.Wait would fail with
// ECHILD.
func TestUntrack_StaleTokenDoesNotRemoveNewerEntry(t *testing.T) {
	const pid = 1 << 30 // never a real child in this test process
	stale := track(pid)
	newer := track(pid)
	Untrack(pid, stale)
	assert.True(t, isTracked(pid), "stale untrack must not expose the newer registration")
	Untrack(pid, newer)
	assert.False(t, isTracked(pid))
}

func TestStartTracked_StartError(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "/nonexistent-binary-for-reaper-test")
	_, err := StartTracked(cmd)
	require.Error(t, err)
}
