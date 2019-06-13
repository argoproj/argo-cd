package testrepos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/errors"
)

type HackPath struct {
	osPath string
}

func (h HackPath) Close() error {
	return os.Setenv("PATH", h.osPath)
}

// add the hack path which has the git-ask-pass.sh shell script
func NewHackPath() HackPath {
	osPath := os.Getenv("PATH")
	binDir, err := filepath.Abs("../../hack")
	errors.CheckError(err)
	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", osPath, binDir))
	errors.CheckError(err)
	return HackPath{osPath}
}
