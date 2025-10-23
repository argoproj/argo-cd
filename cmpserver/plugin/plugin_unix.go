//go:build !windows
// +build !windows

package plugin

import (
	"syscall"
)

func newSysProcAttr(setpgid bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: setpgid}
}

func sysCallKill(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

func sysCallTerm(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
