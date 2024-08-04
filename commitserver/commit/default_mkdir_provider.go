//go:build !linux

package commit

import (
	"fmt"
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
)

func SecureMkdirAll(root, unsafePath string, mode os.FileMode) (string, error) {
	fullPath, err := securejoin.SecureJoin(root, unsafePath)
	if err != nil {
		return "", fmt.Errorf("failed to construct secure path: %w", err)
	}
	err = os.MkdirAll(fullPath, mode)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	return fullPath, nil
}
