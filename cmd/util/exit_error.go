package util

import (
	"fmt"
)

// ExitError represents an error for the CLI, so it can exit with a specific exit code.
// It contains an exit code and an optional error message.
// The non-empty error message logged using errors.Fatal(exitCode, msg) in main.go
type ExitError struct {
	exitCode int
	msg      string // optional
}

func (e *ExitError) ExitCode() int {
	if e == nil {
		return 0
	}
	return e.exitCode
}

func (e *ExitError) Message() string {
	if e == nil {
		return ""
	}
	return e.msg
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("exit error with code %d: %v", e.exitCode, e.msg)
}

// NewExitError creates a new ExitError with the given exit code and optional message.
func NewExitError(exitCode int, msg string) error {
	return &ExitError{exitCode: exitCode, msg: msg}
}
