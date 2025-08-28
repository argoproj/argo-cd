package cache

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterTaintFunctions(t *testing.T) {
	// Start with a clean state by creating a new global taint manager
	globalTaintManager = newClusterTaintManager()
	
	testServer := "https://test-server"
	testGVK := "test.group/v1, Kind=TestResource"
	
	// Test the tainted state functions
	assert.False(t, IsClusterTainted(testServer), "Cluster should not be tainted initially")
	
	// Mark the cluster as tainted
	MarkClusterTainted(testServer, "Test taint reason", testGVK, "TestErrorType")
	
	// Verify the cluster is now tainted
	assert.True(t, IsClusterTainted(testServer), "Cluster should be tainted after marking")
	
	// Get the tainted GVKs
	taintedGVKs := GetTaintedGVKs(testServer)
	assert.Len(t, taintedGVKs, 1, "Should have 1 tainted GVK")
	assert.Equal(t, testGVK, taintedGVKs[0], "Tainted GVK should match")
	
	// Clear the taints
	ClearClusterTaints(testServer)
	
	// Verify cluster is no longer tainted
	assert.False(t, IsClusterTainted(testServer), "Cluster should not be tainted after clearing")
	assert.Empty(t, GetTaintedGVKs(testServer), "Should have no tainted GVKs after clearing")
}

// Test for JSON serialization behavior of the failedResourceGVKs field
func TestFailedResourceGVKsJSONSerialization(t *testing.T) {
	// Define a simple struct that mimics the ClusterCacheInfo
	type TestClusterCacheInfo struct {
		ResourcesCount     int64    `json:"resourcesCount,omitempty"`
		APIsCount          int64    `json:"apisCount,omitempty"`
		LastCacheSyncTime  string   `json:"lastCacheSyncTime,omitempty"`
		// No omitempty tag for failedResourceGVKs - this is what we're testing
		FailedResourceGVKs []string `json:"failedResourceGVKs"`
	}
	
	// Test with a nil slice
	nilCase := TestClusterCacheInfo{
		ResourcesCount:    100,
		APIsCount:         20,
		LastCacheSyncTime: "2025-08-22T00:00:00Z",
		FailedResourceGVKs: nil,
	}
	
	nilJSON, err := json.Marshal(nilCase)
	require.NoError(t, err)
	nilStr := string(nilJSON)
	
	// The field should exist even with nil value
	assert.Contains(t, nilStr, "failedResourceGVKs", "Field should exist in JSON even with nil value")
	
	// Test with an empty slice
	emptyCase := TestClusterCacheInfo{
		ResourcesCount:    100,
		APIsCount:         20,
		LastCacheSyncTime: "2025-08-22T00:00:00Z",
		FailedResourceGVKs: []string{},
	}
	
	emptyJSON, err := json.Marshal(emptyCase)
	require.NoError(t, err)
	emptyStr := string(emptyJSON)
	
	// The field should exist with an empty array
	assert.Contains(t, emptyStr, "failedResourceGVKs", "Field should exist in JSON with empty slice")
	assert.Contains(t, emptyStr, "\"failedResourceGVKs\":[]", "Field should be serialized as empty array")
	
	// Test with values
	withValueCase := TestClusterCacheInfo{
		ResourcesCount:    100,
		APIsCount:         20,
		LastCacheSyncTime: "2025-08-22T00:00:00Z",
		FailedResourceGVKs: []string{"test.group/v1, Kind=TestResource"},
	}
	
	valueJSON, err := json.Marshal(withValueCase)
	require.NoError(t, err)
	valueStr := string(valueJSON)
	
	// The field should exist with the value
	assert.Contains(t, valueStr, "failedResourceGVKs", "Field should exist in JSON with values")
	assert.Contains(t, valueStr, "test.group/v1, Kind=TestResource", "Field should contain the GVK string")
}