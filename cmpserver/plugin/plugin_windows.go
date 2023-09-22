//go:build windows
// +build windows

package plugin

import (
	"syscall"
)

func newSysProcAttr(setpgid bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func sysCallKill(pid int) error {
	return nil
}

func sysCallTerm(pid int) error {
	return nil
}
