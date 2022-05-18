package files

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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

// IsSymlink return true if the given FileInfo relates to a
// symlink file. Returns false otherwise.
func IsSymlink(fi os.FileInfo) bool {
	return fi.Mode()&fs.ModeSymlink == fs.ModeSymlink
}
