//go:build !linux
// +build !linux

package commit

import (
	"os"
)

type DefaultMkdirAllProvider struct{}

func (p *DefaultMkdirAllProvider) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// getMkdirAllProvider returns the default implementation.
func getMkdirAllProvider() MkdirAllProvider {
	return &DefaultMkdirAllProvider{}
}
