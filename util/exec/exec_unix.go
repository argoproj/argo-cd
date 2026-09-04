//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
)

// SetChildProcessGroup puts the command into its own process group, so signals can reach
// grandchildren too. One holding the command's pipes - git-remote-https on a dead connection, say -
// otherwise stalls cmd.Wait long past the timeout.
func SetChildProcessGroup(cmd *exec.Cmd) {
	if !isolateProcessGroups {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// SignalProcessGroup signals the command's whole process group, falling back to the process alone.
// Requires SetChildProcessGroup, which makes the group ID the process ID.
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	// Negative PID addresses the process group. See kill(2).
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil {
		return cmd.Process.Signal(sig)
	}
	return nil
}
