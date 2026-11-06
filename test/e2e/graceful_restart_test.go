package e2e

import (
	"errors"
	"io"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

func checkHealth(t *testing.T, requireHealthy bool) {
	t.Helper()
	resp, err := fixture.DoHttpRequest("GET", "/healthz?full=true", "")
	if requireHealthy {
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	} else {
		if err != nil {
			if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, io.EOF) {
				require.NoErrorf(t, err, "If an error returned, it must be about connection refused, reset by peer or EOF")
			}
		} else {
			require.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, resp.StatusCode)
		}
	}
}

func TestAPIServerGracefulRestart(t *testing.T) {
	fixture.EnsureCleanState(t)

	// Should be healthy.
	checkHealth(t, true)
	// Should trigger API server restart.
	require.NoError(t, fixture.SetParamInSettingConfigMap("url", "http://test-api-server-graceful-restart"))

	// Wait for ~5 seconds
	for range 50 {
		checkHealth(t, false)
		time.Sleep(100 * time.Millisecond)
	}
	// One final time, should be healthy, or restart is considered too slow for tests
	checkHealth(t, true)
	closer, settingsClient, err := fixture.ArgoCDClientset.NewSettingsClient()
	if closer != nil {
		defer closer.Close()
	}
	require.NoError(t, err)
	settings, err := settingsClient.Get(t.Context(), &settings.SettingsQuery{})
	require.NoError(t, err)
	require.Equal(t, "http://test-api-server-graceful-restart", settings.URL)
}
