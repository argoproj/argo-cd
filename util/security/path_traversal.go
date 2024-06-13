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

func isRequestedDirUnderCurrentRoot(currentRoot, requestedPath string) bool {
	if currentRoot == string(filepath.Separator) {
		return true
	} else if currentRoot == requestedPath {
		return true
	}
	if requestedPath[len(requestedPath)-1] != '/' {
		requestedPath = requestedPath + "/"
	}
	if currentRoot[len(currentRoot)-1] != '/' {
		currentRoot = currentRoot + "/"
	}
	return strings.HasPrefix(requestedPath, currentRoot)
}

func parsePath(path string) (string, string) {
	directory := filepath.Dir(path)
	if directory == path {
		return directory, ""
	}
	return directory, filepath.Base(path)
}
