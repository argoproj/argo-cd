package repo

import (
	"os"
	"path/filepath"
	"strings"
)

// returns a formulated temporary directory location to clone a repository
func WorkDir(url string) (string, error) {
	path := filepath.Join(os.TempDir(), strings.Replace(url, "/", "_", -1))
	err := os.Mkdir(path, 0700)
	if err != nil && !os.IsExist(err) {
		return "", err
	}
	return path, nil
}
