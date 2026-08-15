//go:build !windows

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signalDeliveryTimeout bounds how long a test waits for a self-sent signal to cancel the command context.
const signalDeliveryTimeout = 5 * time.Second

func TestWithSignalContext(t *testing.T) {
	tests := []struct {
		name   string
		signal syscall.Signal
	}{
		{"SIGINT cancels the command context", syscall.SIGINT},
		{"SIGTERM cancels the command context", syscall.SIGTERM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := &cobra.Command{}
			command.SetContext(t.Context())

			handlerRan := false
			WithSignalContext(func(c *cobra.Command, _ []string, _ context.CancelFunc) {
				handlerRan = true

				require.NoError(t, syscall.Kill(syscall.Getpid(), tt.signal))

				select {
				case <-c.Context().Done():
					require.ErrorIs(t, c.Context().Err(), context.Canceled)
				case <-time.After(signalDeliveryTimeout):
					t.Errorf("command context was not canceled after receiving %s", tt.signal)
				}
			})(command, nil)

			assert.True(t, handlerRan, "the wrapped handler was never invoked")
		})
	}
}

func TestWithSignalContextE(t *testing.T) {
	tests := []struct {
		name   string
		signal syscall.Signal
	}{
		{"SIGINT cancels the command context", syscall.SIGINT},
		{"SIGTERM cancels the command context", syscall.SIGTERM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := &cobra.Command{}
			command.SetContext(t.Context())

			expectedErr := errors.New("command failed")

			err := WithSignalContextE(func(c *cobra.Command, _ []string, _ context.CancelFunc) error {
				require.NoError(t, syscall.Kill(syscall.Getpid(), tt.signal))

				select {
				case <-c.Context().Done():
					require.ErrorIs(t, c.Context().Err(), context.Canceled)
				case <-time.After(signalDeliveryTimeout):
					t.Errorf("command context was not canceled after receiving %s", tt.signal)
				}

				return expectedErr
			})(command, nil)

			require.ErrorIs(t, err, expectedErr)
		})
	}
}

// The signal-aware context must derive from the command context, otherwise cancellation from the
// caller (and anything carried on that context) is silently dropped.
func TestWithSignalContextDerivesFromCommandContext(t *testing.T) {
	type contextKey string
	const callerKey contextKey = "caller"

	parentCtx, cancelParent := context.WithCancel(context.WithValue(t.Context(), callerKey, "root"))
	defer cancelParent()

	command := &cobra.Command{}
	command.SetContext(parentCtx)

	WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
		assert.Equal(t, []string{"my-app"}, args)
		assert.Equal(t, "root", c.Context().Value(callerKey))

		cancelParent()

		select {
		case <-c.Context().Done():
			require.ErrorIs(t, c.Context().Err(), context.Canceled)
		case <-time.After(signalDeliveryTimeout):
			t.Error("command context was not canceled after the parent context was canceled")
		}
	})(command, []string{"my-app"})
}

// A blocking handler must not trap the user: the wrapper unregisters the signal handler once the first
// signal arrives, so a second signal falls through to the default behaviour and terminates the process.
// This runs in a subprocess because a successful assertion means the process is killed.
func TestWithSignalContextUnregistersAfterFirstSignal(t *testing.T) {
	if os.Getenv("TEST_BLOCKING_HANDLER") == "1" {
		command := &cobra.Command{}
		command.SetContext(t.Context())

		WithSignalContext(func(c *cobra.Command, _ []string, _ context.CancelFunc) {
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			<-c.Context().Done()

			// The wrapper unregisters slightly after the context is canceled, so keep signalling until the
			// default handler takes over and kills us.
			for {
				_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				time.Sleep(10 * time.Millisecond)
			}
		})(command, nil)

		return
	}

	subprocess := exec.CommandContext(t.Context(), os.Args[0], "-test.run="+t.Name(), "-test.timeout=30s")
	subprocess.Env = append(os.Environ(), "TEST_BLOCKING_HANDLER=1")

	exitErr := &exec.ExitError{}
	require.ErrorAs(t, subprocess.Run(), &exitErr)

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	require.True(t, ok, "expected a unix wait status")
	require.True(t, waitStatus.Signaled(), "expected the blocking handler to be killed by a signal, got exit code %d", waitStatus.ExitStatus())
	assert.Equal(t, syscall.SIGINT, waitStatus.Signal())
}
