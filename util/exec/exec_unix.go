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
	return signalProcessGroup(cmd, sig, syscall.Kill)
}

// signalProcessGroup takes the kill syscall, so tests can observe whether the group or the bare
// process was addressed.
func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal, kill func(pid int, sig syscall.Signal) error) error {
	if cmd.Process == nil {
		return nil
	}
	// Without Setpgid the command is not a group leader, so -PID names some unrelated group or none at
	// all - see DisableProcessGroupIsolation. Relying on ESRCH would leave a recycled PID one collision
	// away from signalling someone else's group.
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		return cmd.Process.Signal(sig)
	}
	// Negative PID addresses the process group. See kill(2).
	if err := kill(-cmd.Process.Pid, sig); err != nil {
		return cmd.Process.Signal(sig)
	}
	return nil
}
