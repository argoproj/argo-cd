package util

import (
	"errors"
	"fmt"
)

// ExitError represents an error for the CLI, so it can exit with a specific exit code.
// It contains an exit code and an optional error.
// The error's message will be printed to the user if present otherwise no message will be printed.
type ExitError struct {
	exitCode int
	err      error // optional
}

func (e *ExitError) ExitCode() int {
	if e == nil {
		return 0
	}
	return e.exitCode
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("exit error with code %d: %v", e.exitCode, e.err)
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// NewExitError creates a new ExitError with the given exit code and error.
// When error is present, its message will be used as the error message.
func NewExitError(exitCode int, err error) error {
	return &ExitError{exitCode: exitCode, err: err}
}

// ExitCodeForError returns the exit code for the given error for the CLI to exit with.
// If no error (nil) is provided, then 0 is returned.
// If the error is an error with ExitCode() int method, the result of the method is returned.
// Otherwise, exit code 1 is returned.
func ExitCodeForError(err error) int {
	if err == nil {
		return 0
	}

	if exitErr, ok := errors.AsType[interface {
		error
		ExitCode() int
	}](err); ok {
		return exitErr.ExitCode()
	}

	return 1
}

// CLIMessageForError returns the error message for the given error for the CLI to print before exiting.
// If the error is an ExitError with nil error, then an empty string is returned.
// If the error is an ExitError with a non-nil error, the inner error's message is returned.
// Otherwise, the error message is returned.
func CLIMessageForError(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := errors.AsType[*ExitError](err); ok {
		if e == nil || e.err == nil {
			return "" // no message to be printed
		}
		err = e.err
	}
	return fmt.Sprintf("Error: %v", err)
}
