package app

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/cmpserver/plugin"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// RunningCMPServer starts a CMP server with the given config directory and waits for it to be ready.
// It blocks until the CMP socket is created or times out after 10 seconds.
func (c *Context) RunningCMPServer(configFile string) *Context {
	c.t.Helper()
	startCMPServer(c.t, configFile)
	c.t.Setenv("ARGOCD_BINARY_NAME", "argocd")
	return c
}

// startCMPServer starts the CMP server and waits for its socket to be ready.
// It blocks until the socket file is created or times out after 10 seconds.
func startCMPServer(t *testing.T, configDir string) {
	t.Helper()
	pluginSockFilePath := path.Join(fixture.TmpDir(), fixture.PluginSockFilePath)
	t.Setenv("ARGOCD_BINARY_NAME", "argocd-cmp-server")
	// ARGOCD_PLUGINSOCKFILEPATH should be set as the same value as repo server env var
	t.Setenv("ARGOCD_PLUGINSOCKFILEPATH", pluginSockFilePath)
	if _, err := os.Stat(pluginSockFilePath); os.IsNotExist(err) {
		err := os.Mkdir(pluginSockFilePath, 0o700)
		require.NoError(t, err)
	}

	// Read plugin config to get expected socket path
	cfg, err := plugin.ReadPluginConfig(configDir)
	require.NoError(t, err, "failed to read plugin config from %s", configDir)
	expectedSocket := cfg.Address()

	// Remove stale socket if it exists from a previous test run
	if err := os.Remove(expectedSocket); err != nil && !os.IsNotExist(err) {
		require.NoError(t, err, "failed to remove stale socket")
	}

	// Start CMP server in goroutine (non-blocking)
	go func() {
		errors.NewHandler(t).FailOnErr(fixture.RunWithStdin("", "", "../../dist/argocd", "--config-dir-path", configDir))
	}()

	// Wait for socket to be created
	waitForSocket(t, expectedSocket, 10*time.Second)
}

// waitForSocket polls for a socket file to exist with exponential backoff
func waitForSocket(t *testing.T, socketPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)

	sleepIntervals := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
	}
	sleepIdx := 0

	for time.Now().Before(deadline) {
		if info, err := os.Stat(socketPath); err == nil {
			if info.Mode()&os.ModeSocket != 0 {
				return // Socket exists and is a socket!
			}
		}
		if sleepIdx < len(sleepIntervals) {
			time.Sleep(sleepIntervals[sleepIdx])
			sleepIdx++
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	t.Fatalf("CMP socket %s did not appear within %v", socketPath, timeout)
}
