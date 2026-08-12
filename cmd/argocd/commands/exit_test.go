package commands

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitError_ExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ExitError
		expected int
	}{
		{name: "nil", err: nil, expected: 0},
		{name: "ExitError 1", err: &ExitError{code: 1, err: nil}, expected: 1},
		{name: "ExitError 5", err: &ExitError{code: 5, err: nil}, expected: 5},
		{name: "ExitError 5 with error", err: &ExitError{code: 5, err: errors.New("test")}, expected: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.err.ExitCode()

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExitError_IsSilent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ExitError
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "ExitError silent", err: &ExitError{code: 1, err: nil}, expected: true},
		{name: "ExitError not silent", err: &ExitError{code: 5, err: errors.New("test")}, expected: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.err.IsSilent()

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExitError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ExitError
		expected string
	}{
		{name: "nil", err: nil, expected: ""},
		{name: "ExitError 1", err: &ExitError{code: 1, err: nil}, expected: "exit status 1"},
		{name: "ExitError 5", err: &ExitError{code: 5, err: nil}, expected: "exit status 5"},
		{name: "ExitError 5 with error", err: &ExitError{code: 5, err: errors.New("test")}, expected: "test"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.err.Error()

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExitError_Unwrap(t *testing.T) {
	t.Parallel()

	innerError := errors.New("test")

	tests := []struct {
		name     string
		err      *ExitError
		expected error
	}{
		{name: "nil", err: nil, expected: nil},
		{name: "ExitError no inner", err: &ExitError{code: 1, err: nil}, expected: nil},
		{name: "ExitError inner error", err: &ExitError{code: 5, err: innerError}, expected: innerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := test.err.Unwrap()

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestCLIErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil", err: nil, expected: ""},
		{name: "ExitError silent", err: &ExitError{code: 1, err: nil}, expected: ""},
		{name: "ExitError not silent", err: &ExitError{code: 5, err: errors.New("test")}, expected: "Error: test\n"},
		{name: "plain error", err: errors.New("test"), expected: "Error: test\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := CLIErrorMessage(test.err)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "nil", err: nil, expected: 0},
		{name: "ExitError 1", err: &ExitError{code: 1, err: nil}, expected: 1},
		{name: "ExitError 5 with message", err: &ExitError{code: 5, err: errors.New("test")}, expected: 5},
		{name: "other error", err: errors.New("test"), expected: 1},
		{name: "wrapped ExitError 5", err: fmt.Errorf("wrapped: %w", &ExitError{code: 5, err: errors.New("test")}), expected: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := ExitCodeForError(test.err)

			assert.Equal(t, test.expected, result)
		})
	}
}
