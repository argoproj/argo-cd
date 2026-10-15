package util

import (
	"fmt"
)

// ExitError represents an error for the CLI, so it can exit with a specific exit code without calling os.Exit or log.Fatal in the command itself.
// It contains an exit code and an optional error message.
// Non-empty message is logged with errors.Fatal when created with NewExitError, or printed as-is for NewPlainExitError.
type ExitError struct {
	exitCode  int
	msg       string // optional
	plainText bool
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

// IsPlainText reports whether main should print the message as-is instead of via the logger.
func (e *ExitError) IsPlainText() bool {
	if e == nil {
		return false
	}
	return e.plainText
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("exit error with code %d and message '%v'", e.exitCode, e.msg)
}

// NewExitError creates an ExitError whose optional message is logged with errors.Fatal in main.
func NewExitError(exitCode int, msg string) error {
	return &ExitError{exitCode: exitCode, msg: msg, plainText: false}
}

// NewPlainExitError creates an ExitError whose optional message is printed as-is in main.
func NewPlainExitError(exitCode int, msg string) error {
	return &ExitError{exitCode: exitCode, msg: msg, plainText: true}
}
