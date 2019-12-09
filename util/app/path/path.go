package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Path(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("%s: app path is absolute", path)
	}
	// If this method of creating the appPath is changed, please also change SubtractRelativeFromAbsolutePath in
	// util/security/path_traversal.go
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
