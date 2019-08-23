package fixture

import (
	"io"
	"os"
	"os/exec"

	argoexec "github.com/argoproj/pkg/exec"
)

func Run(workDir, name string, args ...string) (string, error) {
	return RunWithStdin(nil, workDir, name, args...)
}

func RunWithStdin(stdin *io.Reader, workDir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdin != nil {
		cmd.Stdin = *stdin
	}
	cmd.Env = os.Environ()
	cmd.Dir = workDir

	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
}
