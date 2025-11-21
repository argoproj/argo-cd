package admin

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
)

func TestRun_SignalHandling_GracefulShutdown(t *testing.T) {
	stopCalled := false
	d := &dashboard{
		startLocalServer: func(_ context.Context, opts *apiclient.ClientOptions, _ string, _ *int, _ *string, _ clientcmd.ClientConfig) (func(), error) {
			return func() {
				stopCalled = true
				require.True(t, opts.Core, "Core client option should be set to true")
			}, nil
		},
	}

	var runErr error
	doneCh := make(chan struct{})
	go func() {
		runErr = d.Run(t.Context(), &DashboardConfig{ClientOpts: &apiclient.ClientOptions{}})
		close(doneCh)
	}()

	// Allow some time for the dashboard to register the signal handler
	time.Sleep(50 * time.Millisecond)

	proc, procErr := os.FindProcess(os.Getpid())
	require.NoErrorf(t, procErr, "failed to find process: %v", procErr)
	sigErr := proc.Signal(syscall.SIGINT)
	require.NoErrorf(t, sigErr, "failed to send SIGINT: %v", sigErr)

	select {
	case <-doneCh:
		require.NoError(t, runErr)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: dashboard.Run did not exit after SIGINT")
	}

	require.True(t, stopCalled)
}
