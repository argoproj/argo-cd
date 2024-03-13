package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

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
		return CheckSymlinkOutOfBound(absBasePath, path, info)
	})
}

func CheckSymlinkOutOfBound(absBasePath string, path string, info os.FileInfo) error {
	if files.IsSymlink(info) {
		// We don't use filepath.EvalSymlinks because it fails without returning a path
		// if the target doesn't exist.
		linkTarget, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read link %s: %v", path, err)
		}
		// get the path of the symlink relative to basePath, used for error description
		linkRelPath, err := filepath.Rel(absBasePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for symlink: %v", err)
		}
		// deny all absolute symlinks
		if filepath.IsAbs(linkTarget) {
			return &OutOfBoundsSymlinkError{File: linkRelPath}
		}

		return CheckPathOutOfBound(absBasePath, path, linkTarget)
	}
	return nil
}

func CheckPathOutOfBound(basePath string, path string, target string) error {
	log.WithFields(log.Fields{
		"basePath": basePath,
		"path":     path,
		"target":   target,
	}).Debugf("CheckPathOutOfBound called")
	// get the parent directory of the symlink
	// if path is empty, we're not checking a symlink, we don't need the parent dir
	currentDir := filepath.Dir(path)
	if path == "" {
		currentDir = basePath
	}

	// walk each part of the symlink target to make sure it never leaves basePath
	parts := strings.Split(target, string(os.PathSeparator))
	for _, part := range parts {
		newDir := filepath.Join(currentDir, part)
		rel, err := filepath.Rel(basePath, newDir)
		if err != nil {
			log.WithFields(log.Fields{
				"basePath":   basePath,
				"currentDir": currentDir,
				"part":       part,
				"newDir":     newDir,
			}).Errorf("CheckPathOutOfBound: failed to get relative path for symlink target: %v", err)
			return fmt.Errorf("failed to get relative path for symlink target: %v", err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			// return an error so we don't keep traversing the tree
			relPath, _ := filepath.Rel(basePath, path)
			log.WithFields(log.Fields{
				"rel":     rel,
				"relPath": relPath,
			}).Errorf("CheckPathOutOfBound: out of bound file found")
			return &OutOfBoundsSymlinkError{File: relPath}
		}
		currentDir = newDir
	}
	return nil
}
