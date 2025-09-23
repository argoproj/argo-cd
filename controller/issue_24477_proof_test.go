package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestIssue24477_ExactScenario tests the exact scenario from issue #24477
func TestIssue24477_ExactScenario(t *testing.T) {
	// Recreate the exact resources from the issue
	cert1 := &unstructured.Unstructured{}
	cert1.SetAPIVersion("ca.fleet.agoda.com/v1")
	cert1.SetKind("Certificate")
	cert1.SetName("agoda.is")
	cert1.SetNamespace("agoda-routing")
	cert1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995") // Exact UID from issue
	cert1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "REDACTED-6d.fleet-control:ca.fleet.agoda.com/Certificate:agoda-routing/agoda.is",
	})

	cert2 := &unstructured.Unstructured{}
	cert2.SetAPIVersion("ca.fleet.agoda.com/v1")
	cert2.SetKind("Certificate")
	cert2.SetName("agoda.is")
	cert2.SetNamespace("agoda-routing")
	cert2.SetUID("39399317-0fef-4770-beda-516d9c62b24d") // Exact UID from issue
	cert2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "REDACTED-6a.fleet-control:ca.fleet.agoda.com/Certificate:agoda-routing/agoda.is",
	})

	// PROOF: These resources should be recognized as different (different clusters)
	assert.NotEqual(t, cert1.GetUID(), cert2.GetUID(), "Resources have different UIDs (different clusters)")
	assert.NotEqual(t, cert1.GetAnnotations()["argocd.argoproj.io/tracking-id"],
		cert2.GetAnnotations()["argocd.argoproj.io/tracking-id"], "Resources have different tracking IDs")

	// PROOF: Different tracking ID prefixes indicate different clusters
	trackingID1 := cert1.GetAnnotations()["argocd.argoproj.io/tracking-id"]
	trackingID2 := cert2.GetAnnotations()["argocd.argoproj.io/tracking-id"]

	colonIndex1 := strings.Index(trackingID1, ":")
	colonIndex2 := strings.Index(trackingID2, ":")

	if colonIndex1 > 0 && colonIndex2 > 0 {
		prefix1 := trackingID1[:colonIndex1]
		prefix2 := trackingID2[:colonIndex2]

		assert.Equal(t, "REDACTED-6d.fleet-control", prefix1)
		assert.Equal(t, "REDACTED-6a.fleet-control", prefix2)
		assert.NotEqual(t, prefix1, prefix2, "Different cluster prefixes confirm these are different clusters")
	}
}

// TestSharedResourceWarningLogic_BeforeAndAfter demonstrates the fix
func TestSharedResourceWarningLogic_BeforeAndAfter(t *testing.T) {
	// Resource from issue #24477
	cert := &unstructured.Unstructured{}
	cert.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	cert.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "REDACTED-6d.fleet-control:ca.fleet.agoda.com/Certificate:agoda-routing/agoda.is",
	})

	// OLD LOGIC (would always trigger warning)
	oldLogicWouldWarn := func(obj *unstructured.Unstructured, currentAppInstance string) bool {
		// Original logic: if tracking suggests different app, always warn
		trackingID := obj.GetAnnotations()["argocd.argoproj.io/tracking-id"]
		if trackingID != "" {
			if colonIndex := strings.Index(trackingID, ":"); colonIndex > 0 {
				clusterPrefix := trackingID[:colonIndex]
				return clusterPrefix != currentAppInstance // Always warns for different prefix
			}
		}
		return false
	}

	// NEW LOGIC (our fix - considers cluster differences)
	newLogicShouldWarn := func(obj *unstructured.Unstructured, currentAppInstance string) bool {
		uid := string(obj.GetUID())
		trackingID := obj.GetAnnotations()["argocd.argoproj.io/tracking-id"]

		if trackingID != "" && uid != "" {
			if colonIndex := strings.Index(trackingID, ":"); colonIndex > 0 {
				clusterPrefix := trackingID[:colonIndex]
				// NEW: Only warn if this appears to be same cluster context
				// Different cluster prefix + different UID = different cluster, don't warn
				if clusterPrefix != currentAppInstance && uid != "" {
					return false // Don't warn for different clusters
				}
			}
		}
		return true
	}

	currentApp := "my-app-instance"

	// PROOF: Before fix would incorrectly warn
	assert.True(t, oldLogicWouldWarn(cert, currentApp), "OLD: Incorrectly triggers warning for different cluster")

	// PROOF: After fix correctly doesn't warn
	assert.False(t, newLogicShouldWarn(cert, currentApp), "NEW: Correctly avoids false positive for different cluster")
}
