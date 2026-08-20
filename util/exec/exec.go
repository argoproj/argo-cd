package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/tracing"
	"github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/rand"
)

var (
	timeout      time.Duration
	fatalTimeout time.Duration
	Unredacted   = Redact(nil)
)

type ExecRunOpts struct {
	// Redactor redacts tokens from the output
	Redactor func(text string) string
	// TimeoutBehavior configures what to do in case of timeout
	TimeoutBehavior TimeoutBehavior
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
	fatalTimeout, err = time.ParseDuration(os.Getenv("ARGOCD_EXEC_FATAL_TIMEOUT"))
	if err != nil {
		fatalTimeout = 10 * time.Second
	}
}

// defaultCancelGrace applies when the configured grace is not positive: ARGOCD_EXEC_FATAL_TIMEOUT=0
// disables the timeout path's SIGKILL, but cancellation has no other backstop.
const defaultCancelGrace = 10 * time.Second

// CancelGrace is how long a cancelled command has before it is killed. Callers budgeting around
// cancellation must use this rather than the raw ARGOCD_EXEC_FATAL_TIMEOUT, so as not to undercut it.
func CancelGrace() time.Duration {
	if fatalTimeout <= 0 {
		return defaultCancelGrace
	}
	return fatalTimeout
}

// TerminateGroupOnCancel terminates cmd's process group when its context is cancelled: SIGTERM,
// then SIGKILL partway through grace. os/exec instead SIGKILLs the command at once, skipping its cleanup - git
// leaves .git/index.lock behind, and nothing ever reclaims a stale one.
//
// cmd must come from exec.CommandContext. grace bounds the whole sequence as cmd.WaitDelay and falls
// back to defaultCancelGrace; the SIGKILL lands partway through it, see cancelEscalation. Call the
// returned stop once cmd.Wait has returned, so a pending escalation cannot signal a process group that
// has since recycled the PID.
func TerminateGroupOnCancel(cmd *exec.Cmd, grace time.Duration) (stop func()) {
	return terminateGroupOnCancel(cmd, grace, SignalProcessGroup)
}

// cancelEscalation is when the SIGKILL fires within grace. It has to land strictly before WaitDelay,
// which lets Wait return - with a grandchild still holding the inherited pipes, os/exec reports
// ErrWaitDelay at exactly WaitDelay - because callers run stop on that return and it would cancel the
// escalation the surviving group still needs. So the SIGTERM gets the first half of grace to clean up
// and the SIGKILL the second half to take effect, leaving CancelGrace the whole window callers budget
// around.
func cancelEscalation(grace time.Duration) time.Duration {
	return grace / 2
}

// terminateGroupOnCancel takes the signalling function, so tests can observe the escalation.
func terminateGroupOnCancel(cmd *exec.Cmd, grace time.Duration, signal func(*exec.Cmd, syscall.Signal) error) (stop func()) {
	SetChildProcessGroup(cmd)
	if grace <= 0 {
		grace = defaultCancelGrace
	}

	var mu sync.Mutex
	var escalation *time.Timer
	cancelled, stopped := false, false

	cmd.Cancel = func() error {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			// Already waited for: signalling could hit whatever recycled the PID, and reporting an
			// error would fail a command that finished cleanly. os/exec skips both for ErrProcessDone.
			return os.ErrProcessDone
		}
		if cancelled {
			// Already cancelled. os/exec cancels once, but a direct caller signalling again would
			// orphan the timer stop needs to reach.
			return nil
		}
		cancelled = true
		err := signal(cmd, syscall.SIGTERM)
		if errors.Is(err, os.ErrProcessDone) {
			// Already reaped, so there is nothing left to escalate against: an armed timer could
			// only reach whatever recycled the PID.
			return err
		}
		// Reap whatever ignored SIGTERM, as the timeout path does.
		escalation = time.AfterFunc(cancelEscalation(grace), func() {
			mu.Lock()
			defer mu.Unlock()
			if stopped {
				return
			}
			_ = signal(cmd, syscall.SIGKILL)
		})
		// Best effort otherwise: reporting e.g. EPERM here would mask the context error on Wait.
		return nil
	}
	cmd.WaitDelay = grace

	return func() {
		mu.Lock()
		defer mu.Unlock()
		stopped = true
		if escalation != nil {
			escalation.Stop()
		}
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
	cmdOpts := CmdOpts{Timeout: timeout, FatalTimeout: fatalTimeout, Redactor: opts.Redactor, TimeoutBehavior: opts.TimeoutBehavior, SkipErrorLogging: opts.SkipErrorLogging, CaptureStderr: opts.CaptureStderr}
	span := tracing.NewLoggingTracer(log.NewLogrusLogger(log.NewWithCurrentConfig())).StartSpan(fmt.Sprintf("exec %v", cmd.Args[0]))
	span.SetBaggageItem("dir", cmd.Dir)
	if cmdOpts.Redactor != nil {
		span.SetBaggageItem("args", opts.Redactor(fmt.Sprintf("%v", cmd.Args)))
	} else {
		span.SetBaggageItem("args", fmt.Sprintf("%v", cmd.Args))
	}
	defer span.Finish()
	return RunCommandExt(cmd, cmdOpts)
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

type CmdError struct {
	Args   string
	Stderr string
	Cause  error
}

func (ce *CmdError) Error() string {
	res := fmt.Sprintf("`%v` failed %v", ce.Args, ce.Cause)
	if ce.Stderr != "" {
		res = fmt.Sprintf("%s: %s", res, ce.Stderr)
	}
	return res
}

func (ce *CmdError) String() string {
	return ce.Error()
}

func (ce *CmdError) Unwrap() error {
	return ce.Cause
}

func newCmdError(args string, cause error, stderr string) *CmdError {
	return &CmdError{Args: args, Stderr: stderr, Cause: cause}
}

// TimeoutBehavior defines behavior for when the command takes longer than the passed in timeout to exit
// By default, SIGKILL is sent to the process and it is not waited upon
type TimeoutBehavior struct {
	// Signal determines the signal to send to the process
	Signal syscall.Signal
	// ShouldWait determines whether to wait for the command to exit once timeout is reached
	ShouldWait bool
}

type CmdOpts struct {
	// Timeout determines how long to wait for the command to exit
	Timeout time.Duration
	// FatalTimeout is the amount of additional time to wait after Timeout before fatal SIGKILL
	FatalTimeout time.Duration
	// Redactor redacts tokens from the output
	Redactor func(text string) string
	// TimeoutBehavior configures what to do in case of timeout
	TimeoutBehavior TimeoutBehavior
	// SkipErrorLogging defines whether to skip logging of execution errors (rc > 0)
	SkipErrorLogging bool
	// CaptureStderr defines whether to capture stderr in addition to stdout
	CaptureStderr bool
}

var DefaultCmdOpts = CmdOpts{
	Timeout:          time.Duration(0),
	FatalTimeout:     time.Duration(0),
	Redactor:         Unredacted,
	TimeoutBehavior:  TimeoutBehavior{syscall.SIGKILL, false},
	SkipErrorLogging: false,
	CaptureStderr:    false,
}

func Redact(items []string) func(text string) string {
	return func(text string) string {
		for _, item := range items {
			text = strings.ReplaceAll(text, item, "******")
		}
		return text
	}
}

// RunCommandExt is a convenience function to run/log a command and return/log stderr in an error upon
// failure.
func RunCommandExt(cmd *exec.Cmd, opts CmdOpts) (string, error) {
	execId, err := rand.RandHex(5)
	if err != nil {
		return "", err
	}
	logCtx := logrus.WithFields(logrus.Fields{"execID": execId})

	redactor := DefaultCmdOpts.Redactor
	if opts.Redactor != nil {
		redactor = opts.Redactor
	}

	// log in a way we can copy-and-paste into a terminal
	args := strings.Join(cmd.Args, " ")
	logCtx.WithFields(logrus.Fields{"dir": cmd.Dir}).Info(redactor(args))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Own process group, so a timeout reaps grandchildren too. See SignalProcessGroup.
	SetChildProcessGroup(cmd)

	select {
	case <-shutdown:
		return "", ErrShuttingDown
	default:
	}

	start := time.Now()
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	inFlight.Add(1)
	defer inFlight.Add(-1)

	// Buffered: the timeout path returns without reading done when ShouldWait is false, which would
	// otherwise leave this goroutine blocked on the send, holding the command and its output buffers.
	done := make(chan error, 1)
	var reapedMu sync.Mutex
	reaped := false
	go func() {
		waitErr := cmd.Wait()
		reapedMu.Lock()
		reaped = true
		reapedMu.Unlock()
		done <- waitErr
	}()

	// done only becomes readable a moment after Wait returns, so a command can already be reaped - its
	// process group free for reuse - while the select below still sees it as running. Same guard as
	// TerminateGroupOnCancel's stop.
	signalGroup := func(sig syscall.Signal) {
		reapedMu.Lock()
		defer reapedMu.Unlock()
		if reaped {
			return
		}
		_ = SignalProcessGroup(cmd, sig)
	}

	// Start timers for timeout
	timeout := DefaultCmdOpts.Timeout
	fatalTimeout := DefaultCmdOpts.FatalTimeout

	if opts.Timeout != time.Duration(0) {
		timeout = opts.Timeout
	}

	if opts.FatalTimeout != time.Duration(0) {
		fatalTimeout = opts.FatalTimeout
	}

	var timoutCh <-chan time.Time
	if timeout != 0 {
		timoutCh = time.NewTimer(timeout).C
	}

	var fatalTimeoutCh <-chan time.Time
	if fatalTimeout != 0 {
		fatalTimeoutCh = time.NewTimer(timeout + fatalTimeout).C
	}

	finish := func(waitErr error) (string, error) {
		output := stdout.String()
		if opts.CaptureStderr {
			output += stderr.String()
		}
		logCtx.WithFields(logrus.Fields{"duration": time.Since(start)}).Debug(redactor(output))
		if waitErr == nil {
			return strings.TrimSuffix(output, "\n"), nil
		}
		cmdErr := newCmdError(redactor(args), errors.New(redactor(waitErr.Error())), strings.TrimSpace(redactor(stderr.String())))
		if !opts.SkipErrorLogging {
			logCtx.Error(cmdErr.Error())
		}
		return strings.TrimSuffix(output, "\n"), cmdErr
	}

	timeoutBehavior := DefaultCmdOpts.TimeoutBehavior
	fatalTimeoutBehaviour := syscall.SIGKILL
	if opts.TimeoutBehavior.Signal != syscall.Signal(0) {
		timeoutBehavior = opts.TimeoutBehavior
	}

	select {
	// noinspection ALL
	case <-timoutCh:
		// send timeout signal to the whole process group
		_ = SignalProcessGroup(cmd, timeoutBehavior.Signal)
		// wait on timeout signal and fallback to fatal timeout signal
		if timeoutBehavior.ShouldWait {
			select {
			case <-done:
			case <-fatalTimeoutCh:
				// upgrades to SIGKILL if cmd does not respect SIGTERM
				_ = SignalProcessGroup(cmd, fatalTimeoutBehaviour)
				// now original cmd should exit immediately after SIGKILL
				<-done
				// return error with a marker indicating that cmd exited only after fatal SIGKILL
				output := stdout.String()
				if opts.CaptureStderr {
					output += stderr.String()
				}
				logCtx.WithFields(logrus.Fields{"duration": time.Since(start)}).Debug(redactor(output))
				err = newCmdError(redactor(args), fmt.Errorf("fatal timeout after %v", timeout+fatalTimeout), "")
				logCtx.Error(err.Error())
				return strings.TrimSuffix(output, "\n"), err
			}
		}
		// either did not wait for timeout or cmd did respect SIGTERM
		output := stdout.String()
		if opts.CaptureStderr {
			output += stderr.String()
		}
		logCtx.WithFields(logrus.Fields{"duration": time.Since(start)}).Debug(redactor(output))
		err = newCmdError(redactor(args), fmt.Errorf("timeout after %v", timeout), "")
		logCtx.Error(err.Error())
		return strings.TrimSuffix(output, "\n"), err
	case <-shutdown:
		// Both cases can be ready at once, and select picks at random: report a command that has
		// already exited as itself rather than as a shutdown casualty.
		select {
		case waitErr := <-done:
			return finish(waitErr)
		default:
		}
		// Signal the group so the command can clean up, then reap it if it ignores SIGTERM.
		signalGroup(syscall.SIGTERM)
		var waitErr error
		select {
		case waitErr = <-done:
		case <-time.After(CancelGrace()):
			signalGroup(syscall.SIGKILL)
			waitErr = <-done
		}
		if waitErr == nil {
			// Finished on its own between the check above and the SIGTERM, so it is no casualty.
			return finish(nil)
		}
		output := stdout.String()
		if opts.CaptureStderr {
			output += stderr.String()
		}
		logCtx.WithFields(logrus.Fields{"duration": time.Since(start)}).Debug(redactor(output))
		// Keep the exit error behind the sentinel: callers match on its text - git reads "exit status 1"
		// from git diff as "changes found" - and a command that failed on its own during the drain must
		// still report why.
		cause := fmt.Errorf("%w: %w", ErrShuttingDown, errors.New(redactor(waitErr.Error())))
		err = newCmdError(redactor(args), cause, strings.TrimSpace(redactor(stderr.String())))
		if !opts.SkipErrorLogging {
			logCtx.Error(err.Error())
		}
		return strings.TrimSuffix(output, "\n"), err
	case waitErr := <-done:
		return finish(waitErr)
	}
}

func RunCommand(name string, opts CmdOpts, arg ...string) (string, error) {
	return RunCommandExt(exec.CommandContext(context.Background(), name, arg...), opts)
}
