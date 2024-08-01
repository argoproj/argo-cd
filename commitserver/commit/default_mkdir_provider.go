//go:build !linux
// +build !linux

package commit

import (
	"fmt"
	"github.com/cyphar/filepath-securejoin"
	"os"
)

type NonLinuxMkdirAllProvider struct{}

func (p *NonLinuxMkdirAllProvider) MkdirAll(root, hydratePath string, mode os.FileMode) (string, error) {
	fullHydratePath, err := securejoin.SecureJoin(root, hydratePath)
	if err != nil {
		return "", fmt.Errorf("failed to construct hydrate path: %w", err)
	}
	return fullHydratePath, os.MkdirAll(fullHydratePath, mode)
}

func getMkdirAllProvider() MkdirAllProvider {
	return &NonLinuxMkdirAllProvider{}
}
