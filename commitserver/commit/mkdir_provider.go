package commit

import (
	"os"
)

// MkdirAllProvider is an interface for creating directories.
type MkdirAllProvider interface {
	MkdirAll(path string, perm os.FileMode) error
}
