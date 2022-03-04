package files

import (
	"path/filepath"
	"strings"
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
