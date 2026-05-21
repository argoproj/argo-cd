//go:build !windows

package plugin

import (
	"syscall"
)

func newSysProcAttr(setpgid bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: setpgid}
}
