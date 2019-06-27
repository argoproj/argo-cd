package config

import (
	"os"
	"time"

	"github.com/argoproj/pkg/exec"
)

func ExecTimeout() time.Duration {
	duration, _ := time.ParseDuration(os.Getenv("ARGOCD_EXEC_TIMEOUT"))
	return duration
}

func CmdOpts() exec.CmdOpts {
	// TODO - get from above
	return exec.DefaultCmdOpts
}
