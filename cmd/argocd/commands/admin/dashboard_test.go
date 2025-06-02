package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
)

func TestRun_SignalHandling_GracefulShutdown(t *testing.T) {
	stopCalled := false
	cancelCalled := false
	sigCh := make(chan os.Signal, 1)
	d := &dashboard{
		signalChan: sigCh,
		startLocalServer: func(context.Context, *apiclient.ClientOptions, string, *int, *string, clientcmd.ClientConfig) (func(), error) {
			return func() { stopCalled = true }, nil
		},
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(t.Context(), &DashboardConfig{ClientOpts: &apiclient.ClientOptions{}}, func() { cancelCalled = true })
	}()

	time.Sleep(50 * time.Millisecond)

	// emulate sending SIGINT signal into the signal channel
	sigCh <- os.Interrupt

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: dashboard.Run did not exit after SIGINT")
	}

	require.True(t, stopCalled)
	require.True(t, cancelCalled)
}
