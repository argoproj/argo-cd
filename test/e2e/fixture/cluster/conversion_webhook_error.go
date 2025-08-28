package cluster

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/controller/cache"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

const (
	// Mock group, version, kind that will be used to track failure state
	MockGroup   = "mock.argoproj.io"
	MockVersion = "v1alpha1"
	MockKind    = "FailingResource"
)


// VerifyHealthyClusterState checks that the cluster is in a healthy state with no failed resources
func VerifyHealthyClusterState(t *testing.T, clusterServer string) {
	t.Helper()

	// Get cluster info via API
	clusterURL := fixture.URLEncodeServerAddress(clusterServer)
	var cluster v1alpha1.Cluster
	err := fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/"+clusterURL, &cluster)
	require.NoError(t, err)

	// Log the current connection status for debugging
	t.Logf("📊 Cluster connection status: %s", cluster.Info.ConnectionState.Status)
	if cluster.Info.ConnectionState.Message != "" {
		t.Logf("📊 Connection message: %s", cluster.Info.ConnectionState.Message)
	}

	// Convert to JSON to check raw representation
	clusterJSON, err := json.Marshal(cluster)
	require.NoError(t, err)

	// Verify the field exists in the JSON output
	jsonString := string(clusterJSON)
	
	// The failedResourceGVKs field should exist but be empty
	require.Contains(t, jsonString, "failedResourceGVKs")
	
	// Cluster should be successful, not degraded
	require.Equal(t, v1alpha1.ConnectionStatusSuccessful, cluster.Info.ConnectionState.Status,
		"Expected cluster to be in Successful state when healthy")
	
	// The failedResourceGVKs field should be empty
	require.Empty(t, cluster.Info.CacheInfo.FailedResourceGVKs,
		"Expected failedResourceGVKs to be empty for healthy cluster")
	
	t.Logf("📊 Failed resource GVKs: %v (should be empty)", cluster.Info.CacheInfo.FailedResourceGVKs)
}

// VerifyFailedResourcesInResponse checks that the failedResourceGVKs field appears correctly in API responses
// and that the cluster is in degraded state with appropriate error messages
func VerifyFailedResourcesInResponse(t *testing.T, clusterServer string) {
	t.Helper()

	// Get cluster info via API
	clusterURL := fixture.URLEncodeServerAddress(clusterServer)
	var cluster v1alpha1.Cluster
	err := fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/"+clusterURL, &cluster)
	require.NoError(t, err)

	// Log the current connection status for debugging
	t.Logf("📊 Cluster connection status: %s", cluster.Info.ConnectionState.Status)
	if cluster.Info.ConnectionState.Message != "" {
		t.Logf("📊 Connection message: %s", cluster.Info.ConnectionState.Message)
	}

	// Convert to JSON to check raw representation
	clusterJSON, err := json.Marshal(cluster)
	require.NoError(t, err)

	// Verify the field exists in the JSON output
	jsonString := string(clusterJSON)
	
	// The failedResourceGVKs field should exist and not be omitted
	require.Contains(t, jsonString, "failedResourceGVKs")
	
	// Wait up to 30 seconds for cluster to become degraded and failedResourceGVKs to be populated
	var gvks []string
	var isDegraded bool
	var connectionMsg string
	for i := 0; i < 30; i++ {
		// Check if we have both degraded status and failed GVKs
		isDegraded = cluster.Info.ConnectionState.Status == v1alpha1.ConnectionStatusDegraded
		connectionMsg = cluster.Info.ConnectionState.Message
		hasFailedGVKs := len(cluster.Info.CacheInfo.FailedResourceGVKs) > 0
		
		if isDegraded && hasFailedGVKs {
			gvks = cluster.Info.CacheInfo.FailedResourceGVKs
			break
		}
		
		time.Sleep(1 * time.Second)
		// Re-fetch cluster info
		err = fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/"+clusterURL, &cluster)
		require.NoError(t, err)
		
		// Log current status for debugging
		if i%5 == 0 { // Log every 5 seconds
			t.Logf("📊 Waiting for degraded status... Current: status=%s, failedGVKs=%v", 
				cluster.Info.ConnectionState.Status, cluster.Info.CacheInfo.FailedResourceGVKs)
		}
	}
	
	// We should see a degraded status due to conversion webhook failures
	require.Equal(t, v1alpha1.ConnectionStatusDegraded, cluster.Info.ConnectionState.Status, 
		"Expected cluster to be in Degraded state due to conversion webhook failures")
	
	// The message should mention conversion webhook errors or unavailable resource types
	require.True(t, 
		strings.Contains(connectionMsg, "conversion webhook") || strings.Contains(connectionMsg, "unavailable resource types"),
		"Expected connection message to mention conversion webhook or unavailable resource types, got: %s", connectionMsg)
	require.NotEmpty(t, gvks, 
		"Expected failedResourceGVKs to contain failed GVKs after waiting 30 seconds")
	
	// Log the failed GVKs for debugging
	t.Logf("📊 Failed resource GVKs: %v", cluster.Info.CacheInfo.FailedResourceGVKs)
	
	// Should contain the conversion.example.com GVK that we're testing with
	found := false
	for _, gvk := range cluster.Info.CacheInfo.FailedResourceGVKs {
		if strings.Contains(gvk, "conversion.example.com") && strings.Contains(gvk, "Example") {
			found = true
			break
		}
	}
	require.True(t, found, "Expected to find conversion.example.com/Example GVK in failedResourceGVKs: %v", 
		cluster.Info.CacheInfo.FailedResourceGVKs)
}

// ClearMockConversionWebhookFailure removes the mock failure state
func ClearMockConversionWebhookFailure(t *testing.T, clusterServer string) {
	t.Helper()
	cache.ClearClusterTaints(clusterServer)
	t.Log("Cleared mock conversion webhook failure state")
}