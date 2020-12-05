package path

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/util/errors"
)

type AddBinDirToPath struct {
	originalPath string
}

func (h AddBinDirToPath) Close() {
	_ = os.Setenv("PATH", h.originalPath)
}

// add the hack path which has the git-ask-pass.sh shell script
func NewBinDirToPath() AddBinDirToPath {
	originalPath := os.Getenv("PATH")
	binDir, err := filepath.Abs("../../hack")
	errors.CheckError(err)
	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", originalPath, binDir))
	errors.CheckError(err)
	return AddBinDirToPath{originalPath}
}
