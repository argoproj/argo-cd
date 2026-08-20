//go:build windows

package exec

import (
	"os/exec"
	"syscall"
)

// SetChildProcessGroup is a no-op: Windows has no POSIX process groups.
func SetChildProcessGroup(_ *exec.Cmd) {}

// SignalProcessGroup signals the process alone, and always by killing it: os.Process.Signal
// implements no other signal on Windows, so passing one through would report EWINDOWS and leave the
// process running until WaitDelay reaped it. Graceful termination is not available here.
func SignalProcessGroup(cmd *exec.Cmd, _ syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
