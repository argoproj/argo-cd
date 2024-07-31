//go:build linux
// +build linux

package commit

import (
	securejoin "github.com/cyphar/filepath-securejoin"
	"os"
)

type LinuxMkdirAllProvider struct{}

func (p *LinuxMkdirAllProvider) MkdirAll(path string, perm os.FileMode) error {
	return securejoin.MkdirAll(path, perm)
}

// getMkdirAllProvider returns the Linux-specific implementation.
func getMkdirAllProvider() MkdirAllProvider {
	return &LinuxMkdirAllProvider{}
}
