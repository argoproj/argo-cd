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

func SubtractRelativeFromAbsolutePath(abs, rel string) (string, error) {
	if len(rel) == 0 {
		return abs, nil
	}
	if rel[0] == '.' {
		return SubtractRelativeFromAbsolutePath(abs, rel[1:])
	}
	if rel[0] != '/' {
		return SubtractRelativeFromAbsolutePath(abs, "/"+rel)
	}
	if rel[len(rel)-1] == '/' {
		return SubtractRelativeFromAbsolutePath(abs, rel[:len(rel)-1])
	}
	rel = filepath.Clean(rel)
	lastIndex := strings.LastIndex(abs, rel)
	if lastIndex < 0 {
		// This should be unreachable, because by this point the App Path will have already been validated by Path in
		// util/app/path/path.go
		return "", fmt.Errorf("app path is not under repo path (unreachable and most likely a bug)")
	}
	return abs[:lastIndex], nil
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
