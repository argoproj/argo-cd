package commands

import (
	"errors"
	"fmt"
	"os/exec"
)

// ExitError signals to main to exit with a specific exit code.
// It is intended as a return value for RunE function of a cobra command.
// The optional err field contains error that will be used to display a message to the user.
// If err is nil, only exit with the specified exit code.
type ExitError struct {
	code int
	err  error
}

func (e *ExitError) ExitCode() int {
	if e == nil {
		return 0 // nil exit error means no error
	}
	return e.code
}

func (e *ExitError) IsSilent() bool {
	return e != nil && e.err == nil
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit status %d", e.ExitCode())
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func NewExitError(exitCode int, err error) error {
	return &ExitError{
		code: exitCode,
		err:  err,
	}
}

func CLIErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.IsSilent() {
			return ""
		}
	}

	return fmt.Sprintf("Error: %v\n", err)
}

func ExitCodeForError(err error) int {
	if err == nil {
		return 0 // no error
	}

	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		return execErr.ExitCode()
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return 1 // default error exit code
}
