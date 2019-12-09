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

// Determine the original repo path by progressively constructing the appPath the same way it was originally
// done by Path() in util/app/path/path.go until we determine the original repo path
func SubtractRelativeFromAbsolutePath(appPath, rel string) string {
	separatedAbs := strings.Split(appPath, string(filepath.Separator))
	for i := 0; i < len(separatedAbs)+1; i++ {
		possibleRepoPath := strings.Join(separatedAbs[:i], string(filepath.Separator))
		if filepath.Join(possibleRepoPath, rel) == appPath {
			return possibleRepoPath
		}
	}
	return ""
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