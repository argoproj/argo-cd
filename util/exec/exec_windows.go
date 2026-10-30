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
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	if sig == 0 {
		// A liveness probe, not a request to terminate - killing here would reap the very process
		// the caller is asking about. Signal reports ErrProcessDone once reaped and EWINDOWS while
		// the process is alive, which is all the probe needs to tell apart.
		return cmd.Process.Signal(sig)
	}
	return cmd.Process.Kill()
}
