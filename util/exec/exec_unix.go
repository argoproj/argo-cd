//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
)

// setChildProcessGroup puts the command into its own process group so that the
// timeout handler can signal the entire group — the command plus any
// grandchildren it spawned — rather than only the direct child.
//
// Without this, a grandchild that inherited the command's stdout/stderr pipes
// (for example git's git-remote-https helper, which can block on a dead TCP
// connection) keeps those pipes open after the direct child is killed. That
// stalls cmd.Wait() until the grandchild exits on its own, long past the
// configured timeout.
func setChildProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// signalProcessGroup sends sig to the command's whole process group. It relies
// on setChildProcessGroup having made the command a process-group leader, so
// the group ID equals the process ID. If the group signal fails it falls back
// to signalling just the process.
func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	// A negative PID signals the whole process group. See kill(2).
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil {
		return cmd.Process.Signal(sig)
	}
	return nil
}
