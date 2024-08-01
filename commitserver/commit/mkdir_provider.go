package commit

import (
	"os"
)

type MkdirAllProvider interface {
	MkdirAll(root string, unsafePath string, mode os.FileMode) (string, error)
}
