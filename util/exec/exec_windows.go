//go:build windows

package exec

import (
	"os/exec"
	"syscall"
)

// SetChildProcessGroup is a no-op: Windows has no POSIX process groups.
func SetChildProcessGroup(_ *exec.Cmd) {}

// SignalProcessGroup signals the process alone; os.Process.Signal only implements Kill on Windows.
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(sig)
}
