package config

import (
	"os"
	"time"

	"github.com/argoproj/pkg/exec"
)

var timeout time.Duration

func init() {
	initTimeout()
}

func initTimeout() {
	var err error
	timeout, err = time.ParseDuration(os.Getenv("ARGOCD_EXEC_TIMEOUT"))
	if err != nil {
		timeout = 90 * time.Second
	}
}

func CmdOpts() exec.CmdOpts {
	return exec.CmdOpts{Timeout: timeout}
}
