//go:build !windows

package helm

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryIndexLock(file *os.File, shared bool) (bool, error) {
	mode := unix.LOCK_EX
	if shared {
		mode = unix.LOCK_SH
	}
	err := unix.Flock(int(file.Fd()), mode|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}
