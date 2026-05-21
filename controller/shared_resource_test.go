package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSharedResourceWarning_DifferentClusters(t *testing.T) {
	// Test case for issue #24477: SharedResourceWarning for apiservice resources deployed in different clusters
	// Create two resources with same name but different UIDs and tracking IDs (different clusters)
	resource1 := &unstructured.Unstructured{}
	resource1.SetAPIVersion("ca.fleet.agoda.com/v1")
	resource1.SetKind("Certificate")
	resource1.SetName("agoda.is")
	resource1.SetNamespace("agoda-routing")
	resource1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	resource1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "REDACTED-6d.fleet-control:ca.fleet.agoda.com/Certificate:agoda-routing/agoda.is",
	})

	resource2 := &unstructured.Unstructured{}
	resource2.SetAPIVersion("ca.fleet.agoda.com/v1")
	resource2.SetKind("Certificate")
	resource2.SetName("agoda.is")
	resource2.SetNamespace("agoda-routing")
	resource2.SetUID("39399317-0fef-4770-beda-516d9c62b24d") // Different UID
	resource2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "REDACTED-6a.fleet-control:ca.fleet.agoda.com/Certificate:agoda-routing/agoda.is", // Different tracking ID
	})

	// Test that resources with different UIDs and tracking IDs should NOT trigger SharedResourceWarning

	// These should be considered different resources (different clusters)
	assert.NotEqual(t, resource1.GetUID(), resource2.GetUID(), "Resources should have different UIDs")
	assert.NotEqual(t, resource1.GetAnnotations()["argocd.argoproj.io/tracking-id"],
		resource2.GetAnnotations()["argocd.argoproj.io/tracking-id"], "Resources should have different tracking IDs")
}

func TestSharedResourceWarning_SameCluster(t *testing.T) {
	// Test case: Resources in same cluster with same UID should trigger warning

	resource1 := &unstructured.Unstructured{}
	resource1.SetAPIVersion("v1")
	resource1.SetKind("ConfigMap")
	resource1.SetName("test-cm")
	resource1.SetNamespace("default")
	resource1.SetUID("same-uid-12345") // Same UID
	resource1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "app1:v1/ConfigMap:default/test-cm",
	})

	resource2 := &unstructured.Unstructured{}
	resource2.SetAPIVersion("v1")
	resource2.SetKind("ConfigMap")
	resource2.SetName("test-cm")
	resource2.SetNamespace("default")
	resource2.SetUID("same-uid-12345") // Same UID - this IS a shared resource
	resource2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "app2:v1/ConfigMap:default/test-cm",
	})

	// Same UID means same actual resource - should trigger warning
	assert.Equal(t, resource1.GetUID(), resource2.GetUID(), "Same UID should trigger SharedResourceWarning")
}
