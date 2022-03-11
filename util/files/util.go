package files

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var RelativeOutOfBoundErr = errors.New("full path does not contain base path")

// RelativePath will remove the basePath string from the fullPath
// including the path separator. Differently from filepath.Rel, this
// function will return error (RelativeOutOfBoundErr) if basePath
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
//   return:   "", RelativeOutOfBoundErr
//
// Example 3:
//   fullPath: /home/test/app/readme.md
//   basePath: /home/test/app/readme.md
//   return:   .
func RelativePath(fullPath, basePath string) (string, error) {
	fp := filepath.Clean(fullPath)
	if !strings.HasPrefix(fp, basePath) {
		return "", RelativeOutOfBoundErr
	}
	return filepath.Rel(basePath, fp)
}

// CreateTempDir will create a temporary directory with CSPRNG
// entropy in the name to avoid clashes and mitigate directory
// traversal. It is the caller's responsibility to remove the
// directory after use. Will return the full path of the
// generated directory.
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

// IsSymlink return true if the given FileInfo relates to a
// symlink file. Returns false otherwise.
func IsSymlink(fi os.FileInfo) bool {
	return fi.Mode()&fs.ModeSymlink == fs.ModeSymlink
}
