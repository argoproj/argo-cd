package util_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/cmd/util"
)

var testCases = []struct {
	name             string
	err              error
	expectedExitCode int
	expectedMessage  string
}{
	{
		name:             "no error (nil)",
		err:              nil,
		expectedExitCode: 0,
		expectedMessage:  "",
	},
	{
		name:             "error but not an exit error",
		err:              errors.New("test error"),
		expectedExitCode: 1,
		expectedMessage:  "Error: test error",
	},
	{
		name:             "exit error with code 1 and error",
		err:              util.NewExitError(1, errors.New("test error")),
		expectedExitCode: 1,
		expectedMessage:  "Error: test error",
	},
	{
		name:             "exit error with code 1 and nil error",
		err:              util.NewExitError(1, nil),
		expectedExitCode: 1,
		expectedMessage:  "",
	},
	{
		name:             "exit error with code 20 and nil error",
		err:              util.NewExitError(20, nil),
		expectedExitCode: 20,
		expectedMessage:  "",
	},
	{
		name:             "exit error with code 20 and error",
		err:              util.NewExitError(20, errors.New("test error")),
		expectedExitCode: 20,
		expectedMessage:  "Error: test error",
	},
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actualExitCode := util.ExitCodeForError(test.err)

			assert.Equal(t, test.expectedExitCode, actualExitCode)
		})
	}
}

func TestCLIMessageForError(t *testing.T) {
	t.Parallel()

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actualMessage := util.CLIMessageForError(test.err)

			assert.Equal(t, test.expectedMessage, actualMessage)
		})
	}
}

func TestExitError_ExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "nil", err: nil, expected: 0},
		{name: "exit error with code 1 and error", err: util.NewExitError(1, errors.New("test error")), expected: 1},
		{name: "exit error with code 1 and nil error", err: util.NewExitError(1, nil), expected: 1},
		{name: "exit error with code 20 and nil error", err: util.NewExitError(20, nil), expected: 20},
		{name: "exit error with code 20 and error", err: util.NewExitError(20, errors.New("test error")), expected: 20},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitError, _ := errors.AsType[*util.ExitError](test.err)

			actualExitCode := exitError.ExitCode()

			assert.Equal(t, test.expected, actualExitCode)
		})
	}
}

func TestExitError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil", err: nil, expected: ""},
		{name: "exit error with code 2 no error", err: util.NewExitError(2, nil), expected: "exit error with code 2: <nil>"},
		{name: "exit error with code 2 and error", err: util.NewExitError(2, errors.New("test error")), expected: "exit error with code 2: test error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitError, _ := errors.AsType[*util.ExitError](test.err)

			actualError := exitError.Error()

			assert.Equal(t, test.expected, actualError)
		})
	}
}

func TestExitError_Unwrap(t *testing.T) {
	t.Parallel()

	innerError := errors.New("test error")

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{name: "nil", err: nil, expected: nil},
		{name: "exit error with code 2 no error", err: util.NewExitError(2, nil), expected: nil},
		{name: "exit error with code 2 and error", err: util.NewExitError(2, innerError), expected: innerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitError, _ := errors.AsType[*util.ExitError](test.err)

			wrappedError := exitError.Unwrap()

			assert.Equal(t, test.expected, wrappedError)
		})
	}
}
