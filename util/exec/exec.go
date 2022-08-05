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
