package exec

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/v2/util/log"
)

func Run(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	return RunWithRedactor(cmd, nil, timeout)
}

func RunWithRedactor(cmd *exec.Cmd, redactor func(text string) string, timeout time.Duration) (string, error) {
	opts := argoexec.CmdOpts{Timeout: timeout}
	span := tracing.NewLoggingTracer(log.NewLogrusLogger(log.NewWithCurrentConfig())).StartSpan(fmt.Sprintf("exec %v", cmd.Args[0]))
	span.SetBaggageItem("dir", fmt.Sprintf("%v", cmd.Dir))
	if redactor != nil {
		span.SetBaggageItem("args", redactor(fmt.Sprintf("%v", cmd.Args)))
		opts.Redactor = redactor
	} else {
		span.SetBaggageItem("args", fmt.Sprintf("%v", cmd.Args))
	}
	defer span.Finish()
	return argoexec.RunCommandExt(cmd, opts)
}
