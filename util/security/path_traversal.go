package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Ensure that `requestedPath` is on the same directory or any subdirectory of `currentRoot`. Both `currentRoot` and
// `requestedPath` must be absolute paths. They may contain any number of `./` or `/../` dir changes.
func EnforceToCurrentRoot(currentRoot, requestedPath string) (string, error) {
	currentRoot = filepath.Clean(currentRoot)
	requestedDir, requestedFile := parsePath(requestedPath)
	if !isRequestedDirUnderCurrentRoot(currentRoot, requestedDir) {
		return "", fmt.Errorf("requested path %s should be on or under current directory %s", requestedPath, currentRoot)
	}
	return requestedDir + string(filepath.Separator) + requestedFile, nil
}

func SubtractRelativeFromAbsolutePath(abs, rel string) string {
	if len(rel) == 0 {
		return abs
	}
	if rel[0] == '.' {
		rel = rel[1:]
	}
	if rel[0] != '/' {
		rel = "/" + rel
	}
	if rel[len(rel)-1] == '/' {
		rel = rel[:len(rel)-1]
	}
	return abs[:strings.LastIndex(abs, rel)]
}

func isRequestedDirUnderCurrentRoot(currentRoot, requestedDir string) bool {
	if currentRoot == string(filepath.Separator) {
		return true
	} else if currentRoot == requestedDir {
		return true
	}
	return strings.HasPrefix(requestedDir, currentRoot+string(filepath.Separator))
}

func parsePath(path string) (string, string) {
	directory := filepath.Dir(path)
	if directory == path {
		return directory, ""
	}
	return directory, filepath.Base(path)
}
