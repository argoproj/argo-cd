package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/io/files"
)

func Path(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("%s: app path is absolute", path)
	}
	appPath := filepath.Join(root, path)
	if !strings.HasPrefix(appPath, filepath.Clean(root)) {
		return "", fmt.Errorf("%s: app path outside root", path)
	}
	info, err := os.Stat(appPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s: app path does not exist", path)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: app path is not a directory", path)
	}
	return appPath, nil
}

type OutOfBoundsSymlinkError struct {
	File string
	Err  error
}

func (e *OutOfBoundsSymlinkError) Error() string {
	return "out of bounds symlink found"
}

// CheckOutOfBoundsSymlinks determines if basePath contains any symlinks that
// are absolute or point to a path outside of the basePath. If found, an
// OutOfBoundsSymlinkError is returned.
func CheckOutOfBoundsSymlinks(basePath string) error {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	return filepath.Walk(absBasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk for symlinks in %s: %v", absBasePath, err)
		}
		if files.IsSymlink(info) {
			return CheckOutOfBoundsSingleSymlink(path, absBasePath)
		}
		return nil
	})
}

// CheckOutOfBoundsSymlink determines if path is a symlink that
// point to a path outside of the basePath. If this is the case, an
// OutOfBoundsSymlinkError is returned.
func CheckOutOfBoundsSymlink(path string, basePath string) error {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return err
	}
	if files.IsSymlink(info) {
		return CheckOutOfBoundsSingleSymlink(absPath, absBasePath)
	}
	return nil

}

// CheckOutOfBoundsSingleSymlink determines if the symlink at the absolute path absPath
// points to a path outside of the absBasePath. If this is the case, an
// OutOfBoundsSymlinkError is returned.
func CheckOutOfBoundsSingleSymlink(absPath string, absBasePath string) error {
	// We don't use filepath.EvalSymlinks because it fails without returning a path
	// if the target doesn't exist.
	linkTarget, err := os.Readlink(absPath)
	if err != nil {
		return fmt.Errorf("failed to read link %s: %v", absPath, err)
	}
	// get the path of the symlink relative to basePath, used for error description
	linkRelPath, err := filepath.Rel(absBasePath, absPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path for symlink: %v", err)
	}
	// deny all absolute symlinks
	if filepath.IsAbs(linkTarget) {
		return &OutOfBoundsSymlinkError{File: linkRelPath}
	}
	// get the parent directory of the symlink
	currentDir := filepath.Dir(absPath)

	// walk each part of the symlink target to make sure it never leaves basePath
	parts := strings.Split(linkTarget, string(os.PathSeparator))
	for _, part := range parts {
		newDir := filepath.Join(currentDir, part)
		rel, err := filepath.Rel(absBasePath, newDir)
		if err != nil {
			return fmt.Errorf("failed to get relative path for symlink target: %v", err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			// return an error so we don't keep traversing the tree
			return &OutOfBoundsSymlinkError{File: linkRelPath}
		}
		currentDir = newDir
	}
	return nil
}
