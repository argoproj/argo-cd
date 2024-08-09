package exec

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	// SkipErrorLogging determines whether to skip logging of execution errors (rc > 0)
	SkipErrorLogging bool
	// CaptureStderr determines whether to capture stderr in addition to stdout
	CaptureStderr bool
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
	cmdOpts := argoexec.CmdOpts{Timeout: timeout, Redactor: opts.Redactor, TimeoutBehavior: opts.TimeoutBehavior, SkipErrorLogging: opts.SkipErrorLogging}
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

// GetCommandArgsToLog represents the given command in a way that we can copy-and-paste into a terminal
func GetCommandArgsToLog(cmd *exec.Cmd) string {
	var argsToLog []string
	for _, arg := range cmd.Args {
		if arg == "" {
			argsToLog = append(argsToLog, `""`)
			continue
		}

		containsSpace := false
		for _, r := range arg {
			if unicode.IsSpace(r) {
				containsSpace = true
				break
			}
		}
		if containsSpace {
			// add quotes and escape any internal quotes
			argsToLog = append(argsToLog, strconv.Quote(arg))
		} else {
			argsToLog = append(argsToLog, arg)
		}
	}
	args := strings.Join(argsToLog, " ")
	return args
}
