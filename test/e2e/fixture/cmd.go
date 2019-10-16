package fixture

import (
	"os"
	"os/exec"
	"strings"

	argoexec "github.com/argoproj/pkg/exec"
)

func Run(workDir, name string, args ...string) (string, error) {
	return RunWithStdin("", workDir, name, args...)
}

func RunWithStdin(stdin, workDir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Env = os.Environ()
	cmd.Dir = workDir

	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
}

func RunWithVars(workDir string, env []string, name string, args ...string) (string, error) {
	return RunWithStdinVars("", workDir, env, name, args...)
}

func RunWithStdinVars(stdin, workDir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	cmd.Env = os.Environ()
	cmd.Dir = workDir

	for _, v := range env {
		cmd.Env = append(cmd.Env, v)
	}

	return argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
}
