package fixture

import (
	"os"
	"os/exec"

	argoexec "github.com/argoproj/pkg/exec"
)

func Run(workDir, name string, args ...string) (string, error) {

	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	cmd.Dir = workDir

	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
}
