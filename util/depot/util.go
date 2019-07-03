package depot

import (
	"os"
	"path/filepath"
	"strings"
)

// returns a formulated temporary directory location to clone a repository
func TempRepoPath(repo string) string {
	path := filepath.Join(os.TempDir(), strings.Replace(repo, "/", "_", -1))
	_ = os.Mkdir(path, 0777)
	return path
}
