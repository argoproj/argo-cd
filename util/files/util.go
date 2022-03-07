package files

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// RelativePath will remove the basePath string from the fullPath
// including the path separator. Will return fullPath if basePath
// does not match (example 2).
//
// Example 1:
//   fullPath: /home/test/app/readme.md
//   basePath: /home/test
//   return:   app/readme.md
//
// Example 2:
//   fullPath: /home/test/app/readme.md
//   basePath: /somewhere/else
//   return:   /home/test/app/readme.md
//
// Example 3:
//   fullPath: /home/test/app/readme.md
//   basePath: /home/test/app/readme.md
//   return:   .
func RelativePath(fullPath, basePath string) string {
	replaced := strings.Replace(fullPath, basePath, "", 1)
	if replaced == fullPath {
		return fullPath
	}
	trimmed := strings.TrimPrefix(replaced, string(filepath.Separator))
	return filepath.Clean(trimmed)
}

// CreateTempDir will create a temporary directory with CSPRNG
// entropy in the name to avoid clashes. It is the caller's
// responsibility to remove the directory after use. Will return
// the full path of the generated directory.
func CreateTempDir() (string, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	tempDir := path.Join(os.TempDir(), newUUID.String())
	if err := os.Mkdir(tempDir, 0755); err != nil {
		return "", err
	}
	return tempDir, nil
}
