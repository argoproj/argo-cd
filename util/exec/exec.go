package exec

import (
	"context"
	"fmt"
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

func Run(ctx context.Context, cmd *exec.Cmd) (string, error) {
	return RunWithRedactor(ctx, cmd, nil)
}
func RunWithRedactor(ctx context.Context, cmd *exec.Cmd, redactor func(text string) string) (string, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, fmt.Sprintf("exec %v", cmd.Args[0]))
	span.SetBaggageItem("dir", fmt.Sprintf("%v", cmd.Dir))
	span.SetBaggageItem("args", fmt.Sprintf("%v", cmd.Args))
	defer span.Finish()
	opts := argoexec.CmdOpts{Timeout: timeout}
	if redactor != nil {
		opts.Redactor = redactor
	}
	return argoexec.RunCommandExt(cmd, opts)
}
