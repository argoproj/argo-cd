//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetChildProcessGroupIsolationDisabled covers the argocd CLI: a command moved out of the
// terminal's foreground process group stops receiving Ctrl-C.
func TestSetChildProcessGroupIsolationDisabled(t *testing.T) {
	t.Cleanup(func() { isolateProcessGroups = true })
	DisableProcessGroupIsolation()

	cmd := exec.CommandContext(t.Context(), "true")
	SetChildProcessGroup(cmd)
	assert.Nil(t, cmd.SysProcAttr)

	isolateProcessGroups = true
	SetChildProcessGroup(cmd)
	require.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setpgid)
}

// TestSignalProcessGroupSkipsGroupWithoutSetpgid covers the CLI, where the command shares the caller's
// process group: -PID would then name an unrelated group, and a recycled PID is all it takes for that
// group to exist and be signalled.
func TestSignalProcessGroupSkipsGroupWithoutSetpgid(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "sleep 30")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	var killed []int
	record := func(pid int, _ syscall.Signal) error {
		killed = append(killed, pid)
		return nil
	}

	// No SetChildProcessGroup: nothing may be addressed by negative PID.
	require.NoError(t, signalProcessGroup(cmd, syscall.SIGTERM, record))
	assert.Empty(t, killed, "signalled a process group the command does not lead")

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, signalProcessGroup(cmd, syscall.SIGTERM, record))
	assert.Equal(t, []int{-cmd.Process.Pid}, killed)
}
