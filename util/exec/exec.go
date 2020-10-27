package exec

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util/log"

	argoexec "github.com/argoproj/pkg/exec"

	tracing "github.com/argoproj/gitops-engine/pkg/utils/tracing"
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
	span := tracing.NewLoggingTracer(log.NewLogrusLogger(logrus.New())).StartSpan(fmt.Sprintf("exec %v", cmd.Args[0]))
	span.SetBaggageItem("dir", fmt.Sprintf("%v", cmd.Dir))
	span.SetBaggageItem("args", fmt.Sprintf("%v", cmd.Args))
	defer span.Finish()
	opts := argoexec.CmdOpts{Timeout: timeout}
	if redactor != nil {
		opts.Redactor = redactor
	}
	return argoexec.RunCommandExt(cmd, opts)
}
