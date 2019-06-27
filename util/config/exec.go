package config

import (
	"os"
	"time"

	"github.com/argoproj/pkg/exec"
)

func timeout() time.Duration {
	duration, err := time.ParseDuration(os.Getenv("ARGOCD_EXEC_TIMEOUT"))
	if err != nil {
		duration = 90 * time.Second
	}
	return duration
}

func CmdOpts() exec.CmdOpts {
	return exec.CmdOpts{Timeout: timeout()}
}
