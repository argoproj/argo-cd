package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnforceToCurrentRoot opens the file securely, ensuring it stays within currentRoot.
// It returns the open *os.File handle. The caller is responsible for closing the file.
func EnforceToCurrentRoot(currentRoot, requestedPath string) (*os.File, error) {
	// 1. Create the secure jail (os.Root).
	// os.OpenRoot guarantees that any operations performed on 'root' are confined to that directory.
	root, err := os.OpenRoot(currentRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create root at %s: %w", currentRoot, err)
	}
	// Closing the root handle does NOT close files opened from it.
	// We can safely close it before returning.
	defer root.Close()

	// 2. Normalize paths for the relative path calculation.
	// We use filepath.Clean on the string inputs ONLY to calculate the relative path
	// for the lookup. We do not use these strings for the actual OS-level access.
	cleanRoot := filepath.Clean(currentRoot)
	cleanPath := filepath.Clean(requestedPath)

	relPath, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return nil, fmt.Errorf("requested path %s should be on or under current directory %s", requestedPath, currentRoot)
	}

	// 3. Lexical Check (Defense in Depth).
	// Fast-fail if the path obviously tries to traverse upwards.
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("requested path %s should be on or under current directory %s", requestedPath, currentRoot)
	}

	// 4. Secure Open
	// We open the file relative to the secure root handle.
	// If 'relPath' attempts to escape the root (even via symlinks), the OS will block it.
	f, err := root.Open(relPath)
	if err != nil {
		return nil, fmt.Errorf("access denied or file not found within root: %w", err)
	}

	return f, nil
}
