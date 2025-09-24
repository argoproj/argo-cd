package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// TestSharedResourceWarningAllTrackingMethods tests the fix for issue #24477
// across all supported tracking methods
func TestSharedResourceWarningAllTrackingMethods(t *testing.T) {
	tests := []struct {
		name           string
		trackingMethod string
		setupResources func() (*unstructured.Unstructured, *unstructured.Unstructured)
		expectWarning  bool
		description    string
	}{
		{
			name:           "annotation_tracking_different_clusters",
			trackingMethod: "annotation",
			setupResources: createAnnotationTrackedResources,
			expectWarning:  false,
			description:    "Resources with different tracking ID prefixes should not trigger warning",
		},
		{
			name:           "annotation_tracking_same_cluster",
			trackingMethod: "annotation",
			setupResources: createSameClusterAnnotationResources,
			expectWarning:  true,
			description:    "Resources with same tracking ID prefix should trigger warning",
		},
		{
			name:           "label_tracking_different_clusters",
			trackingMethod: "label",
			setupResources: createLabelTrackedResources,
			expectWarning:  false,
			description:    "Resources with different cluster-aware labels should not trigger warning",
		},
		{
			name:           "annotation_plus_label_tracking",
			trackingMethod: "annotation+label",
			setupResources: createAnnotationPlusLabelResources,
			expectWarning:  false,
			description:    "Mixed tracking method should work correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res1, _ := tt.setupResources()

			// Create mock app state manager
			manager := createMockAppStateManager()
			app := createMockApplication()

			// Test the logic
			shouldWarn := manager.shouldTriggerSharedResourceWarning(
				res1,
				getAppInstanceName(res1, tt.trackingMethod),
				app,
				tt.trackingMethod,
			)

			if tt.expectWarning {
				assert.True(t, shouldWarn, tt.description)
			} else {
				assert.False(t, shouldWarn, tt.description)
			}
		})
	}
}

func createAnnotationTrackedResources() (*unstructured.Unstructured, *unstructured.Unstructured) {
	// Cluster A resource
	res1 := &unstructured.Unstructured{}
	res1.SetAPIVersion("apps/v1")
	res1.SetKind("Deployment")
	res1.SetName("nginx")
	res1.SetNamespace("default")
	res1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	res1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "cluster-a-app:apps/Deployment:default/nginx",
	})

	// Cluster B resource
	res2 := &unstructured.Unstructured{}
	res2.SetAPIVersion("apps/v1")
	res2.SetKind("Deployment")
	res2.SetName("nginx")
	res2.SetNamespace("default")
	res2.SetUID("39399317-0fef-4770-beda-516d9c62b24d")
	res2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "cluster-b-app:apps/Deployment:default/nginx",
	})

	return res1, res2
}

func createSameClusterAnnotationResources() (*unstructured.Unstructured, *unstructured.Unstructured) {
	// Same cluster, different apps - should trigger warning
	res1 := &unstructured.Unstructured{}
	res1.SetAPIVersion("apps/v1")
	res1.SetKind("Deployment")
	res1.SetName("nginx")
	res1.SetNamespace("default")
	res1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	res1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "other-app:apps/Deployment:default/nginx", // Different app
	})

	res2 := &unstructured.Unstructured{}
	res2.SetAPIVersion("apps/v1")
	res2.SetKind("Deployment")
	res2.SetName("nginx")
	res2.SetNamespace("default")
	res2.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995") // Same UID = same cluster
	res2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "test-app:apps/Deployment:default/nginx", // Current app
	})

	return res1, res2
}

func createLabelTrackedResources() (*unstructured.Unstructured, *unstructured.Unstructured) {
	res1 := &unstructured.Unstructured{}
	res1.SetAPIVersion("apps/v1")
	res1.SetKind("Deployment")
	res1.SetName("nginx")
	res1.SetNamespace("default")
	res1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	res1.SetLabels(map[string]string{
		"app.kubernetes.io/instance": "cluster-a-app",
	})

	res2 := &unstructured.Unstructured{}
	res2.SetAPIVersion("apps/v1")
	res2.SetKind("Deployment")
	res2.SetName("nginx")
	res2.SetNamespace("default")
	res2.SetUID("39399317-0fef-4770-beda-516d9c62b24d")
	res2.SetLabels(map[string]string{
		"app.kubernetes.io/instance": "cluster-b-app",
	})

	return res1, res2
}

func createAnnotationPlusLabelResources() (*unstructured.Unstructured, *unstructured.Unstructured) {
	res1 := &unstructured.Unstructured{}
	res1.SetAPIVersion("apps/v1")
	res1.SetKind("Deployment")
	res1.SetName("nginx")
	res1.SetNamespace("default")
	res1.SetUID("62e7a834-97c6-4a99-8abf-8bbcb1dec995")
	res1.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "cluster-a-app:apps/Deployment:default/nginx",
	})
	res1.SetLabels(map[string]string{
		"app.kubernetes.io/instance": "cluster-a-app",
	})

	res2 := &unstructured.Unstructured{}
	res2.SetAPIVersion("apps/v1")
	res2.SetKind("Deployment")
	res2.SetName("nginx")
	res2.SetNamespace("default")
	res2.SetUID("39399317-0fef-4770-beda-516d9c62b24d")
	res2.SetAnnotations(map[string]string{
		"argocd.argoproj.io/tracking-id": "cluster-b-app:apps/Deployment:default/nginx",
	})
	res2.SetLabels(map[string]string{
		"app.kubernetes.io/instance": "cluster-b-app",
	})

	return res1, res2
}

// Helper functions for testing
func createMockAppStateManager() *appStateManager {
	// Return a minimal mock for testing
	return &appStateManager{
		namespace: "argocd",
	}
}

func createMockApplication() *v1alpha1.Application {
	app := &v1alpha1.Application{}
	app.Name = "test-app"
	app.Namespace = "argocd"
	return app
}

func getAppInstanceName(resource *unstructured.Unstructured, trackingMethod string) string {
	switch trackingMethod {
	case "annotation", "annotation+label":
		if annotations := resource.GetAnnotations(); annotations != nil {
			if trackingID := annotations["argocd.argoproj.io/tracking-id"]; trackingID != "" {
				if colonIndex := strings.Index(trackingID, ":"); colonIndex > 0 {
					return trackingID[:colonIndex]
				}
			}
		}
	case "label":
		if labels := resource.GetLabels(); labels != nil {
			return labels["app.kubernetes.io/instance"]
		}
	}
	return ""
}
