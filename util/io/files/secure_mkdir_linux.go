//go:build linux

package files

import (
	"fmt"
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// SecureMkdirAll creates a directory with the given mode and returns the full path to the directory. It prevents
// directory traversal attacks by ensuring the path is within the root directory. The path is constructed as if the
// given root is the root of the filesystem. So anything traversing outside the root is simply removed from the path.
func SecureMkdirAll(root, unsafePath string, mode os.FileMode) (string, error) {
	err := securejoin.MkdirAll(root, unsafePath, int(mode))
	if err != nil {
		return "", fmt.Errorf("failed to make directory: %w", err)
	}
	fullPath, err := securejoin.SecureJoin(root, unsafePath)
	if err != nil {
		return "", fmt.Errorf("failed to construct secure path: %w", err)
	}
	return fullPath, nil
}
