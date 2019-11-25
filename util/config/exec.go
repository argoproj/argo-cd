package config

import (
	"context"
	"os"
	"os/exec"
	"time"

	argoexec "github.com/argoproj/pkg/exec"
	"github.com/opentracing/opentracing-go"
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

func RunCommandExt(ctx context.Context, cmd *exec.Cmd, opts argoexec.CmdOpts) (string, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "exec "+cmd.Path+" "+cmd.Args[0])
	defer span.Finish()
	return argoexec.RunCommandExt(cmd, opts)
}

func CmdOpts() argoexec.CmdOpts {
	return argoexec.CmdOpts{Timeout: timeout}
}
