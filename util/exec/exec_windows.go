//go:build windows

package exec

import (
	"os/exec"
	"syscall"
)

// SetChildProcessGroup is a no-op on Windows, which has no POSIX process
// groups. Timeout handling falls back to signalling the process directly.
func SetChildProcessGroup(_ *exec.Cmd) {}

// SignalProcessGroup signals the process directly on Windows, preserving the
// previous best-effort behaviour (os.Process.Signal only meaningfully supports
// Kill on Windows).
func SignalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(sig)
}
