package util_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/cmd/util"
)

func TestExitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		err              error
		expectedExitCode int
		expectedMessage  string
		expectedError    string
	}{
		{
			name:             "nil",
			err:              nil,
			expectedExitCode: 0,
			expectedMessage:  "",
			expectedError:    "",
		},
		{
			name:             "typed nil",
			err:              (*util.ExitError)(nil),
			expectedExitCode: 0,
			expectedMessage:  "",
			expectedError:    "",
		},
		{
			name:             "code 1 with message",
			err:              util.NewExitError(1, "test error"),
			expectedExitCode: 1,
			expectedMessage:  "test error",
			expectedError:    "exit error with code 1 and message 'test error'",
		},
		{
			name:             "code 1 with empty message",
			err:              util.NewExitError(1, ""),
			expectedExitCode: 1,
			expectedMessage:  "",
			expectedError:    "exit error with code 1 and message ''",
		},
		{
			name:             "code 20 with empty message",
			err:              util.NewExitError(20, ""),
			expectedExitCode: 20,
			expectedMessage:  "",
			expectedError:    "exit error with code 20 and message ''",
		},
		{
			name:             "code 20 with message",
			err:              util.NewExitError(20, "test error"),
			expectedExitCode: 20,
			expectedMessage:  "test error",
			expectedError:    "exit error with code 20 and message 'test error'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitError, _ := errors.AsType[*util.ExitError](test.err)

			assert.Equal(t, test.expectedExitCode, exitError.ExitCode())
			assert.Equal(t, test.expectedMessage, exitError.Message())
			assert.Equal(t, test.expectedError, exitError.Error())
		})
	}
}

func TestExitError_Wrapped(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("write failed: %w", util.NewExitError(20, "permission denied"))

	exitError, ok := errors.AsType[*util.ExitError](wrapped)
	require.True(t, ok)
	assert.Equal(t, 20, exitError.ExitCode())
	assert.Equal(t, "permission denied", exitError.Message())
}
