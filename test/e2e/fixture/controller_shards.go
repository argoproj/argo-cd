package fixture

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

// Scaffolding for e2e tests that need more than one application controller shard
// running at once. The Procfile hands the controller command to
// test/e2e/fixture/run-controllers.sh, which starts one controller process per
// shard listed in EnvE2EControllerShards. Without that variable it starts a
// single controller, so the normal single-shard stack is unaffected.
const (
	// EnvE2EControllerShards is a comma-separated list of shard numbers to run.
	EnvE2EControllerShards = "ARGOCD_E2E_CONTROLLER_SHARDS"
	// EnvE2EControllerMetricsPortBase is the port shard 0 serves metrics on.
	// Each subsequent shard listens one port higher.
	EnvE2EControllerMetricsPortBase = "ARGOCD_E2E_CONTROLLER_METRICS_PORT_BASE"
	// ControllerShardMetricsPortBase avoids the ports the single-shard stack
	// already uses for controller (8082) and API server (8083) metrics.
	ControllerShardMetricsPortBase = 18082
)

// StartControllerShards replaces the single application controller with one
// process per shard and waits until every shard serves metrics. The single
// controller is restored when the test finishes.
//
// Each shard reads the cluster secrets and Applications it owns from the same
// cluster, so a test that wants an Application handled by a specific shard
// should point it at a cluster assigned to that shard.
func StartControllerShards(t *testing.T, replicas int) {
	t.Helper()

	shards := make([]string, replicas)
	for shard := range shards {
		shards[shard] = strconv.Itoa(shard)
	}

	require.NoError(t, RestartProcess(ApplicationControllerProcName, map[string]string{
		common.EnvControllerReplicas:    strconv.Itoa(replicas),
		EnvE2EControllerShards:          strings.Join(shards, ","),
		EnvE2EControllerMetricsPortBase: strconv.Itoa(ControllerShardMetricsPortBase),
	}))
	t.Cleanup(func() {
		require.NoError(t, RestartProcess(ApplicationControllerProcName, nil))
	})

	for shard := range replicas {
		require.Eventually(t, func() bool {
			return controllerShardIsReady(ControllerShardMetricsPortBase + shard)
		}, time.Minute, 250*time.Millisecond, "controller shard %d did not become ready", shard)
	}
}

// ControllerShardLogPath is where run-controllers.sh sends a shard's output.
// Shards share a terminal, so their logs are split into a file each.
func ControllerShardLogPath(shard int) string {
	return fmt.Sprintf("/tmp/argocd-e2e-controller-shard-%d.log", shard)
}

func controllerShardIsReady(metricsPort int) bool {
	client := http.Client{Timeout: time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://localhost:%d/metrics", metricsPort), http.NoBody)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
