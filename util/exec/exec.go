package exec

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/v2/util/log"
)

var timeout time.Duration

type ExecRunOpts struct {
	// Redactor redacts tokens from the output
	Redactor func(text string) string
	// TimeoutBehavior configures what to do in case of timeout
	TimeoutBehavior argoexec.TimeoutBehavior
}

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

func Run(cmd *exec.Cmd) (string, error) {
	return RunWithRedactor(cmd, nil)
}

func RunWithRedactor(cmd *exec.Cmd, redactor func(text string) string) (string, error) {
	opts := ExecRunOpts{Redactor: redactor}
	return RunWithExecRunOpts(cmd, opts)
}

func RunWithExecRunOpts(cmd *exec.Cmd, opts ExecRunOpts) (string, error) {
	cmdOpts := argoexec.CmdOpts{Timeout: timeout, Redactor: opts.Redactor, TimeoutBehavior: opts.TimeoutBehavior}
	span := tracing.NewLoggingTracer(log.NewLogrusLogger(log.NewWithCurrentConfig())).StartSpan(fmt.Sprintf("exec %v", cmd.Args[0]))
	span.SetBaggageItem("dir", fmt.Sprintf("%v", cmd.Dir))
	if cmdOpts.Redactor != nil {
		span.SetBaggageItem("args", opts.Redactor(fmt.Sprintf("%v", cmd.Args)))
	} else {
		span.SetBaggageItem("args", fmt.Sprintf("%v", cmd.Args))
	}
	defer span.Finish()
	return argoexec.RunCommandExt(cmd, cmdOpts)
}
