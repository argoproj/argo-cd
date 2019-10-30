package exec

import (
	"os/exec"

	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/config"
)

func Run(dir, cmd string, args ...string) string {
	command := exec.Command(cmd, args...)
	command.Dir = dir
	output, err := argoexec.RunCommandExt(command, config.CmdOpts())
	errors.CheckError(err)
	return output
}
