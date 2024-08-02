//go:build linux
// +build linux

package commit

import (
	"github.com/cyphar/filepath-securejoin"
)

func SecureMkdirAll(root, unsafePath string, mode os.FileMode) (string, error) {
	return "", securejoin.MkdirAll(root, unsafePath, mode)
}
