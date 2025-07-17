package path

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/v2/util/errors"
)

type AddBinDirToPath struct {
	originalPath string
}

func (h AddBinDirToPath) Close() {
	_ = os.Setenv("PATH", h.originalPath)
}

// add the hack path which has the argocd binary
func NewBinDirToPath() AddBinDirToPath {
	originalPath := os.Getenv("PATH")
	binDir, err := filepath.Abs("../../dist")
	errors.CheckError(err)
	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", originalPath, binDir))
	errors.CheckError(err)
	return AddBinDirToPath{originalPath}
}
