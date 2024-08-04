//go:build linux

package commit

import (
	"fmt"

	"github.com/cyphar/filepath-securejoin"
)

func SecureMkdirAll(root, unsafePath string, mode os.FileMode) (string, error) {
	err := securejoin.MkdirAll(root, unsafePath, mode)
	if err != nil {
		return "", fmt.Errorf("failed to make directory: %w", err)
	}
	fullPath, err := securejoin.SecureJoin(root, unsafePath)
	if err != nil {
		return "", fmt.Errorf("failed to construct secure path: %w", err)
	}
	return fullPath, nil
}
