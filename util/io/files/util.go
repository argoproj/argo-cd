package files

import (
	"errors"
	"fmt"
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
//
//	fullPath: /home/test/app/readme.md
//	basePath: /home/test
//	return:   app/readme.md
//
// Example 2:
//
//	fullPath: /home/test/app/readme.md
//	basePath: /somewhere/else
//	return:   "", RelativeOutOfBoundErr
//
// Example 3:
//
//	fullPath: /home/test/app/readme.md
//	basePath: /home/test/app/readme.md
//	return:   .
func RelativePath(fullPath, basePath string) (string, error) {
	fp := filepath.Clean(fullPath)
	if !strings.HasPrefix(fp, filepath.Clean(basePath)) {
		return "", RelativeOutOfBoundErr
	}
	return filepath.Rel(basePath, fp)
}

// CreateTempDir will create a temporary directory in baseDir
// with CSPRNG entropy in the name to avoid clashes and mitigate
// directory traversal. If baseDir is empty string, os.TempDir()
// will be used. It is the caller's responsibility to remove the
// directory after use. Will return the full path of the generated
// directory.
func CreateTempDir(baseDir string) (string, error) {
	base := baseDir
	if base == "" {
		base = os.TempDir()
	}
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("error creating directory name: %w", err)
	}
	tempDir := path.Join(base, newUUID.String())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("error creating tempDir: %w", err)
	}
	return tempDir, nil
}

// IsSymlink return true if the given FileInfo relates to a
// symlink file. Returns false otherwise.
func IsSymlink(fi os.FileInfo) bool {
	return fi.Mode()&fs.ModeSymlink == fs.ModeSymlink
}

// Inbound will validate if the given candidate path is inside the
// baseDir. This is useful to make sure that malicious candidates
// are not targeting a file outside of baseDir boundaries.
// Considerations:
// - baseDir must be absolute path. Will return false otherwise
// - candidate can be absolute or relative path
// - candidate should not be symlink as only syntatic validation is
// applied by this function
func Inbound(candidate, baseDir string) bool {
	if !filepath.IsAbs(baseDir) {
		return false
	}
	var target string
	if filepath.IsAbs(candidate) {
		target = filepath.Clean(candidate)
	} else {
		target = filepath.Join(baseDir, candidate)
	}
	return strings.HasPrefix(target, filepath.Clean(baseDir)+string(os.PathSeparator))
}
