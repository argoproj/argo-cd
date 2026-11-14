package admin

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
)

func TestDashboardCommand_SignalHandling_GracefulShutdown(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	d := &dashboard{
		startLocalServer: func(_ context.Context, opts *apiclient.ClientOptions, _ string, _ *int, _ *string, _ clientcmd.ClientConfig) (func(), error) {
			assert.True(t, opts.Core, "Core client option should be set to true")
			close(started)
			return func() { close(stopped) }, nil
		},
	}

	cmd := newDashboardCommand(d, &apiclient.ClientOptions{})
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.ExecuteContext(t.Context())
	}()

	// the signal handler is registered before the command body runs, so a started server means it is in place
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: local server was never started")
	}

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(syscall.SIGINT))

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: dashboard command did not exit after SIGINT")
	}

	select {
	case <-stopped:
	default:
		t.Fatal("shutdown func was not called")
	}
}
