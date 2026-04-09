package commands

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Test data helpers

// createTestApp creates a test application
func createTestApp(name, namespace string, sources ...v1alpha1.ApplicationSource) *v1alpha1.Application {
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
		},
	}

	if len(sources) == 1 {
		app.Spec.Source = &sources[0]
	} else if len(sources) > 1 {
		app.Spec.Sources = sources
	}

	return app
}

// createTestProject creates a test project
func createTestProject(name, namespace string) *v1alpha1.AppProject {
	return &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// createTestUnstructured converts a Kubernetes runtime.Object to unstructured
func createTestUnstructured(obj any) *unstructured.Unstructured {
	return kube.MustToUnstructured(obj)
}

// Mock implementations for testing

// mockManifestProvider creates a mock manifestProvider that returns the given manifests
func mockManifestProvider(manifests []*unstructured.Unstructured) manifestProvider {
	return func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		return manifests, nil
	}
}

// mockDiffStrategy creates a mock diffStrategy that marks all items as modified
func mockDiffStrategyAllModified() diffStrategy {
	return func(ctx context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
		results := make([]*diff.DiffResult, len(items))
		for i, item := range items {
			liveBytes, _ := json.Marshal(item.live)
			targetBytes, _ := json.Marshal(item.target)
			results[i] = &diff.DiffResult{
				Modified:      true,
				NormalizedLive: liveBytes,
				PredictedLive:  targetBytes,
			}
		}
		return results, nil
	}
}

// mockDiffStrategyNoneModified creates a mock diffStrategy that marks no items as modified
func mockDiffStrategyNoneModified() diffStrategy {
	return func(ctx context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
		results := make([]*diff.DiffResult, len(items))
		for i := range items {
			results[i] = &diff.DiffResult{
				Modified: false,
			}
		}
		return results, nil
	}
}

// Test cases for computeDiff

// TestComputeDiff_DefaultCase tests the default case with both live and target resources
func TestComputeDiff_DefaultCase(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	// Create test resources with both live and target states
	liveDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})
	targetDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test", "version": "v2"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{liveDeployment})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{targetDeployment})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "test-deployment", results[0].key.Name)
	assert.NotNil(t, results[0].live)
	assert.NotNil(t, results[0].target)
}

// TestComputeDiff_AddedResource tests when a resource exists in target but not live
func TestComputeDiff_AddedResource(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	// Create a target-only resource (added) - no live state
	targetDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{targetDeployment})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "new-deployment", results[0].key.Name)
	assert.Nil(t, results[0].live)
	assert.NotNil(t, results[0].target)
}

// TestComputeDiff_RemovedResource tests when a resource exists in live but not target
func TestComputeDiff_RemovedResource(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	// Create a live-only resource (removed) - no target state
	liveDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{liveDeployment})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "old-deployment", results[0].key.Name)
	assert.NotNil(t, results[0].live)
	assert.Nil(t, results[0].target)
}

// TestComputeDiff_MultipleResources tests handling multiple resources
func TestComputeDiff_MultipleResources(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	// Create multiple resources
	liveDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})
	targetDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test", "version": "v2"},
		},
	})

	liveService := createTestUnstructured(&corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})
	targetService := createTestUnstructured(&corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{liveDeployment, liveService})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{targetDeployment, targetService})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 2)

	// Verify both resources are present
	deploymentFound := false
	serviceFound := false
	for _, result := range results {
		if result.key.Kind == "Deployment" && result.key.Name == "test-deployment" {
			deploymentFound = true
			assert.NotNil(t, result.live)
			assert.NotNil(t, result.target)
		}
		if result.key.Kind == "Service" && result.key.Name == "test-service" {
			serviceFound = true
			assert.NotNil(t, result.live)
			assert.NotNil(t, result.target)
		}
	}
	assert.True(t, deploymentFound, "Deployment should be found")
	assert.True(t, serviceFound, "Service should be found")
}

// TestComputeDiff_EmptyResources tests with no resources
func TestComputeDiff_EmptyResources(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	assert.Empty(t, results)
}

// TestComputeDiff_MixedAddedRemovedModified tests a scenario with added, removed, and modified resources
func TestComputeDiff_MixedAddedRemovedModified(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	// Modified resource (exists in both live and target)
	liveDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "modified-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})

	// Removed resource (exists only in live)
	liveService := createTestUnstructured(&corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "removed-service",
			Namespace: "default",
		},
	})

	// Added resource (exists only in target)
	addedConfigMap := createTestUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "added-configmap",
			Namespace: "default",
		},
	})

	// Modified resource target
	targetDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "modified-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test", "version": "v2"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{liveDeployment, liveService})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{targetDeployment, addedConfigMap})
	performDiff := mockDiffStrategyAllModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify we have all three types
	modifiedFound := false
	removedFound := false
	addedFound := false

	for _, result := range results {
		switch result.key.Name {
		case "modified-deployment":
			modifiedFound = true
			assert.NotNil(t, result.live)
			assert.NotNil(t, result.target)
		case "removed-service":
			removedFound = true
			assert.NotNil(t, result.live)
			assert.Nil(t, result.target)
		case "added-configmap":
			addedFound = true
			assert.Nil(t, result.live)
			assert.NotNil(t, result.target)
		}
	}

	assert.True(t, modifiedFound, "Modified deployment should be found")
	assert.True(t, removedFound, "Removed service should be found")
	assert.True(t, addedFound, "Added configmap should be found")
}

// TestComputeDiff_NoModifications tests that resources without modifications are not returned
func TestComputeDiff_NoModifications(t *testing.T) {
	ctx := context.Background()
	app := createTestApp("test-app", "argocd", v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/test/repo",
		Path:    "manifests",
	})

	liveDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})
	targetDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
	})

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{liveDeployment})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{targetDeployment})
	performDiff := mockDiffStrategyNoneModified()

	results, err := computeDiff(ctx, app, getLiveManifests, getTargetManifests, performDiff)

	require.NoError(t, err)
	// No modifications, so no results
	assert.Empty(t, results)
}

// Test helper functions

// TestManifestsToUnstructured tests the manifestsToUnstructured helper function
func TestManifestsToUnstructured(t *testing.T) {
	t.Run("Empty manifests", func(t *testing.T) {
		result, err := manifestsToUnstructured([]string{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Single manifest", func(t *testing.T) {
		deployment := createTestUnstructured(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
		})
		deploymentBytes, _ := json.Marshal(deployment)

		result, err := manifestsToUnstructured([]string{string(deploymentBytes)})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Deployment", result[0].GetKind())
		assert.Equal(t, "test-deployment", result[0].GetName())
	})

	t.Run("Multiple manifests", func(t *testing.T) {
		deployment := createTestUnstructured(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
		})
		service := createTestUnstructured(&corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
			},
		})

		deploymentBytes, _ := json.Marshal(deployment)
		serviceBytes, _ := json.Marshal(service)

		result, err := manifestsToUnstructured([]string{string(deploymentBytes), string(serviceBytes)})
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Deployment", result[0].GetKind())
		assert.Equal(t, "Service", result[1].GetKind())
	})

	t.Run("Invalid manifest", func(t *testing.T) {
		result, err := manifestsToUnstructured([]string{"invalid json"})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
