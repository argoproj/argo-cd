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
		name                string
		err                 error
		expectedExitCode    int
		expectedMessage     string
		expectedError       string
		expectedIsPlainText bool
	}{
		{
			name:                "nil",
			err:                 nil,
			expectedExitCode:    0,
			expectedMessage:     "",
			expectedError:       "",
			expectedIsPlainText: false,
		},
		{
			name:                "typed nil",
			err:                 (*util.ExitError)(nil),
			expectedExitCode:    0,
			expectedMessage:     "",
			expectedError:       "",
			expectedIsPlainText: false,
		},
		{
			name:                "code 1 with message",
			err:                 util.NewExitError(1, "test error"),
			expectedExitCode:    1,
			expectedMessage:     "test error",
			expectedError:       "exit error with code 1 and message 'test error'",
			expectedIsPlainText: false,
		},
		{
			name:                "code 1 with empty message",
			err:                 util.NewExitError(1, ""),
			expectedExitCode:    1,
			expectedMessage:     "",
			expectedError:       "exit error with code 1 and message ''",
			expectedIsPlainText: false,
		},
		{
			name:                "code 20 with empty message",
			err:                 util.NewExitError(20, ""),
			expectedExitCode:    20,
			expectedMessage:     "",
			expectedError:       "exit error with code 20 and message ''",
			expectedIsPlainText: false,
		},
		{
			name:                "code 20 with message",
			err:                 util.NewExitError(20, "test error"),
			expectedExitCode:    20,
			expectedMessage:     "test error",
			expectedError:       "exit error with code 20 and message 'test error'",
			expectedIsPlainText: false,
		},
		{
			name:                "code 20 with message and plain text",
			err:                 util.NewPlainExitError(20, "test error"),
			expectedExitCode:    20,
			expectedMessage:     "test error",
			expectedError:       "exit error with code 20 and message 'test error'",
			expectedIsPlainText: true,
		},
		{
			name:                "code 1 with empty message and plain text",
			err:                 util.NewPlainExitError(1, ""),
			expectedExitCode:    1,
			expectedMessage:     "",
			expectedError:       "exit error with code 1 and message ''",
			expectedIsPlainText: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exitError, _ := errors.AsType[*util.ExitError](test.err)

			assert.Equal(t, test.expectedExitCode, exitError.ExitCode())
			assert.Equal(t, test.expectedMessage, exitError.Message())
			assert.Equal(t, test.expectedError, exitError.Error())
			assert.Equal(t, test.expectedIsPlainText, exitError.IsPlainText())
		})
	}
}

func TestExitError_Wrapped(t *testing.T) {
	t.Parallel()

	t.Run("new exit error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("write failed: %w", util.NewExitError(20, "permission denied"))

		exitError, ok := errors.AsType[*util.ExitError](wrapped)
		require.True(t, ok)
		assert.Equal(t, 20, exitError.ExitCode())
		assert.Equal(t, "permission denied", exitError.Message())
		assert.False(t, exitError.IsPlainText())
	})

	t.Run("new plain exit error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("write failed: %w", util.NewPlainExitError(20, "permission denied"))

		exitError, ok := errors.AsType[*util.ExitError](wrapped)
		require.True(t, ok)
		assert.Equal(t, 20, exitError.ExitCode())
		assert.Equal(t, "permission denied", exitError.Message())
		assert.True(t, exitError.IsPlainText())
	})
}
