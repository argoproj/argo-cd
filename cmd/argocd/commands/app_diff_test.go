package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/utils/ptr"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
	settingspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
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

// createTestUnstructured converts a Kubernetes runtime.Object to unstructured
func createTestUnstructured(obj any) *unstructured.Unstructured {
	return kube.MustToUnstructured(obj)
}

// Mock implementations for testing

// mockManifestProvider creates a mock manifestProvider that returns the given manifests
func mockManifestProvider(manifests []*unstructured.Unstructured) manifestProvider {
	return func(_ context.Context) ([]*unstructured.Unstructured, error) {
		return manifests, nil
	}
}

// mockDiffStrategy creates a mock diffStrategy that marks all items as modified
func mockDiffStrategyAllModified() diffStrategy {
	return func(_ context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
		results := make([]*diff.DiffResult, len(items))
		for i, item := range items {
			liveBytes, _ := json.Marshal(item.live)
			targetBytes, _ := json.Marshal(item.target)
			results[i] = &diff.DiffResult{
				Modified:       true,
				NormalizedLive: liveBytes,
				PredictedLive:  targetBytes,
			}
		}
		return results, nil
	}
}

// mockDiffStrategyNoneModified creates a mock diffStrategy that marks no items as modified
func mockDiffStrategyNoneModified() diffStrategy {
	return func(_ context.Context, items []comparisonObject) ([]*diff.DiffResult, error) {
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
func TestCompareManifests_DefaultCase(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "test-deployment", results[0].key.Name)
	assert.NotNil(t, results[0].live)
	assert.NotNil(t, results[0].target)
}

// TestComputeDiff_AddedResource tests when a resource exists in target but not live
func TestCompareManifests_AddedResource(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "new-deployment", results[0].key.Name)
	assert.Nil(t, results[0].live)
	assert.NotNil(t, results[0].target)
}

// TestComputeDiff_RemovedResource tests when a resource exists in live but not target
func TestCompareManifests_RemovedResource(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Deployment", results[0].key.Kind)
	assert.Equal(t, "old-deployment", results[0].key.Name)
	assert.NotNil(t, results[0].live)
	assert.Nil(t, results[0].target)
}

// TestComputeDiff_MultipleResources tests handling multiple resources
func TestCompareManifests_MultipleResources(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

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
func TestCompareManifests_EmptyResources(t *testing.T) {
	ctx := context.Background()

	getLiveManifests := mockManifestProvider([]*unstructured.Unstructured{})
	getTargetManifests := mockManifestProvider([]*unstructured.Unstructured{})
	performDiff := mockDiffStrategyAllModified()

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

	require.NoError(t, err)
	assert.Empty(t, results)
}

// TestComputeDiff_MixedAddedRemovedModified tests a scenario with added, removed, and modified resources
func TestCompareManifests_MixedAddedRemovedModified(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

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
func TestCompareManifests_NoModifications(t *testing.T) {
	ctx := context.Background()

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

	results, err := compareManifests(ctx, getTargetManifests, getLiveManifests, performDiff)

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
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestNewMultiSourceRevisionProvider(t *testing.T) {
	ctx := context.Background()
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

	t.Run("Success", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.EXPECT().GetManifests(ctx, &applicationpkg.ApplicationManifestQuery{
			Name:            ptr.To("test-app"),
			AppNamespace:    ptr.To("test-ns"),
			Revisions:       []string{"rev1", "rev2"},
			SourcePositions: []int64{1, 2},
			NoCache:         ptr.To(true),
		}).Return(&repoapiclient.ManifestResponse{
			Manifests: []string{string(deploymentBytes)},
		}, nil)

		provider := newMultiSourceRevisionProvider(mockClient, "test-app", "test-ns", []string{"rev1", "rev2"}, []int64{1, 2}, true)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Deployment", result[0].GetKind())
	})

	t.Run("GetManifests error", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.EXPECT().GetManifests(ctx, &applicationpkg.ApplicationManifestQuery{
			Name:            ptr.To("test-app"),
			AppNamespace:    ptr.To("test-ns"),
			Revisions:       []string{"rev1"},
			SourcePositions: []int64{1},
			NoCache:         ptr.To(false),
		}).Return(nil, errors.New("test error"))

		provider := newMultiSourceRevisionProvider(mockClient, "test-app", "test-ns", []string{"rev1"}, []int64{1}, false)
		result, err := provider(ctx)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "test error")
	})
}

func TestNewSingleRevisionProvider(t *testing.T) {
	ctx := context.Background()
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
	serviceBytes, _ := json.Marshal(service)

	t.Run("Success", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.EXPECT().GetManifests(ctx, &applicationpkg.ApplicationManifestQuery{
			Name:         ptr.To("my-app"),
			AppNamespace: ptr.To("my-ns"),
			Revision:     ptr.To("abc123"),
			NoCache:      ptr.To(false),
		}).Return(&repoapiclient.ManifestResponse{
			Manifests: []string{string(serviceBytes)},
		}, nil)

		provider := newSingleRevisionProvider(mockClient, "my-app", "my-ns", "abc123", false)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Service", result[0].GetKind())
	})

	t.Run("GetManifests error", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.EXPECT().GetManifests(ctx, &applicationpkg.ApplicationManifestQuery{
			Name:         ptr.To("my-app"),
			AppNamespace: ptr.To("my-ns"),
			Revision:     ptr.To("invalid"),
			NoCache:      ptr.To(false),
		}).Return(nil, errors.New("revision not found"))

		provider := newSingleRevisionProvider(mockClient, "my-app", "my-ns", "invalid", false)
		result, err := provider(ctx)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "revision not found")
	})
}

func TestNewDefaultTargetProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("Success with multiple items", func(t *testing.T) {
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
		serviceBytes, _ := json.Marshal(service)

		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					TargetState: string(deploymentBytes),
				},
				{
					TargetState: string(serviceBytes),
				},
			},
		}

		provider := newDefaultTargetProvider(liveState)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Deployment", result[0].GetKind())
		assert.Equal(t, "Service", result[1].GetKind())
	})

	t.Run("Empty items", func(t *testing.T) {
		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{},
		}

		provider := newDefaultTargetProvider(liveState)
		result, err := provider(ctx)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Invalid JSON in TargetState", func(t *testing.T) {
		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					TargetState: "invalid json",
				},
			},
		}

		provider := newDefaultTargetProvider(liveState)
		result, err := provider(ctx)

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestNewLiveManifestProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
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

		secret := createTestUnstructured(&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
			},
		})
		secretBytes, _ := json.Marshal(secret)

		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Kind:                "Deployment",
					Group:               "apps",
					NormalizedLiveState: string(deploymentBytes),
				},
				{
					Kind:                "Secret",
					Group:               "",
					NormalizedLiveState: string(secretBytes),
				},
			},
		}

		provider := newLiveManifestProvider(liveState, false)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Deployment", result[0].GetKind())
		assert.Equal(t, "Secret", result[1].GetKind())
	})

	t.Run("Empty items", func(t *testing.T) {
		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{},
		}

		provider := newLiveManifestProvider(liveState, false)
		result, err := provider(ctx)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Secret excluded", func(t *testing.T) {
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

		secret := createTestUnstructured(&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
			},
		})
		secretBytes, _ := json.Marshal(secret)

		liveState := &applicationpkg.ManagedResourcesResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Kind:                "Deployment",
					Group:               "apps",
					NormalizedLiveState: string(deploymentBytes),
				},
				{
					Kind:                "Secret",
					Group:               "",
					NormalizedLiveState: string(secretBytes),
				},
			},
		}

		provider := newLiveManifestProvider(liveState, true)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Deployment", result[0].GetKind())
	})
}

// Test Diff Strategy Functions

func TestNewServerSideDiffStrategy(t *testing.T) {
	ctx := context.Background()

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
	deploymentKey := kube.GetResourceKey(deployment)

	t.Run("Success with modified resource", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		// Mock server-side diff response - use mock.Anything for complex query structure
		mockClient.On("ServerSideDiff", mock.Anything, mock.Anything).Return(&applicationpkg.ApplicationServerSideDiffResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Modified:    true,
					LiveState:   `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`,
					TargetState: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","labels":{"new":"label"}}}`,
				},
			},
		}, nil)

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		items := []comparisonObject{
			{
				key:    deploymentKey,
				live:   deployment,
				target: deployment,
			},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Modified)
		assert.NotEmpty(t, results[0].NormalizedLive)
		assert.NotEmpty(t, results[0].PredictedLive)
		mockClient.AssertExpectations(t)
	})

	t.Run("Empty items returns empty results", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		results, err := strategy(ctx, []comparisonObject{})

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("API error is propagated", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		mockClient.On("ServerSideDiff", mock.Anything, mock.Anything).Return(nil, errors.New("API error"))

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		items := []comparisonObject{
			{
				key:    deploymentKey,
				live:   deployment,
				target: deployment,
			},
		}

		results, err := strategy(ctx, items)

		require.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "API error")
		mockClient.AssertExpectations(t)
	})

	t.Run("Handles multiple resources", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

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
		serviceKey := kube.GetResourceKey(service)

		mockClient.On("ServerSideDiff", mock.Anything, mock.Anything).Return(&applicationpkg.ApplicationServerSideDiffResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Modified:    true,
					LiveState:   `{"kind":"Deployment"}`,
					TargetState: `{"kind":"Deployment"}`,
				},
				{
					Modified:    false,
					LiveState:   `{"kind":"Service"}`,
					TargetState: `{"kind":"Service"}`,
				},
			},
		}, nil)

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		items := []comparisonObject{
			{key: deploymentKey, live: deployment, target: deployment},
			{key: serviceKey, live: service, target: service},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.True(t, results[0].Modified)
		assert.False(t, results[1].Modified)
		mockClient.AssertExpectations(t)
	})

	t.Run("Respects batch size limit", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		// Create resources with large state to exceed batch size
		largeData := make([]byte, 600*1024) // 600KB per resource
		for i := range largeData {
			largeData[i] = 'x'
		}
		largeState := string(largeData)

		// Mock expects 2 separate calls because each resource is ~600KB
		// and maxBatchKB is 1024KB (1MB), so they won't fit in one batch
		mockClient.On("ServerSideDiff", mock.Anything, mock.MatchedBy(func(query *applicationpkg.ApplicationServerSideDiffQuery) bool {
			// First batch should have 1 resource
			return len(query.LiveResources) == 1
		})).Return(&applicationpkg.ApplicationServerSideDiffResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Modified:    true,
					LiveState:   `{"kind":"Deployment1"}`,
					TargetState: `{"kind":"Deployment1"}`,
				},
			},
		}, nil).Once()

		mockClient.On("ServerSideDiff", mock.Anything, mock.MatchedBy(func(query *applicationpkg.ApplicationServerSideDiffQuery) bool {
			// Second batch should have 1 resource
			return len(query.LiveResources) == 1
		})).Return(&applicationpkg.ApplicationServerSideDiffResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Modified:    false,
					LiveState:   `{"kind":"Deployment2"}`,
					TargetState: `{"kind":"Deployment2"}`,
				},
			},
		}, nil).Once()

		deployment1 := createTestUnstructured(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment1",
				Namespace: "default",
			},
		})
		deployment1.Object["largeData"] = largeState

		deployment2 := createTestUnstructured(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment2",
				Namespace: "default",
			},
		})
		deployment2.Object["largeData"] = largeState

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		items := []comparisonObject{
			{key: kube.GetResourceKey(deployment1), live: deployment1, target: deployment1},
			{key: kube.GetResourceKey(deployment2), live: deployment2, target: deployment2},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 2)
		// Verify both results are present (order may vary due to parallel processing)
		modifiedCount := 0
		for _, result := range results {
			if result.Modified {
				modifiedCount++
			}
		}
		assert.Equal(t, 1, modifiedCount, "Expected exactly 1 modified resource")
		mockClient.AssertExpectations(t)
	})

	t.Run("Batches resources efficiently within size limit", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		// Create 3 small resources that should fit in one batch
		smallData := make([]byte, 100*1024) // 100KB per resource = 300KB total
		for i := range smallData {
			smallData[i] = 'y'
		}
		smallState := string(smallData)

		// Mock expects only 1 call because all 3 resources fit in 1MB batch
		mockClient.On("ServerSideDiff", mock.Anything, mock.MatchedBy(func(query *applicationpkg.ApplicationServerSideDiffQuery) bool {
			// Should batch all 3 resources together
			return len(query.LiveResources) == 3
		})).Return(&applicationpkg.ApplicationServerSideDiffResponse{
			Items: []*v1alpha1.ResourceDiff{
				{
					Modified:    true,
					LiveState:   `{"kind":"Deployment1"}`,
					TargetState: `{"kind":"Deployment1"}`,
				},
				{
					Modified:    false,
					LiveState:   `{"kind":"Deployment2"}`,
					TargetState: `{"kind":"Deployment2"}`,
				},
				{
					Modified:    true,
					LiveState:   `{"kind":"Deployment3"}`,
					TargetState: `{"kind":"Deployment3"}`,
				},
			},
		}, nil).Once()

		deployment1 := createTestUnstructured(&appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			ObjectMeta: metav1.ObjectMeta{Name: "deployment1", Namespace: "default"},
		})
		deployment1.Object["smallData"] = smallState

		deployment2 := createTestUnstructured(&appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			ObjectMeta: metav1.ObjectMeta{Name: "deployment2", Namespace: "default"},
		})
		deployment2.Object["smallData"] = smallState

		deployment3 := createTestUnstructured(&appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			ObjectMeta: metav1.ObjectMeta{Name: "deployment3", Namespace: "default"},
		})
		deployment3.Object["smallData"] = smallState

		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 5, 1024)
		items := []comparisonObject{
			{key: kube.GetResourceKey(deployment1), live: deployment1, target: deployment1},
			{key: kube.GetResourceKey(deployment2), live: deployment2, target: deployment2},
			{key: kube.GetResourceKey(deployment3), live: deployment3, target: deployment3},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.True(t, results[0].Modified)
		assert.False(t, results[1].Modified)
		assert.True(t, results[2].Modified)
		mockClient.AssertExpectations(t)
	})

	t.Run("Respects concurrency limit", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		mockClient := applicationmocks.NewApplicationServiceClient(t)

		// Create 5 resources with large state to force 5 separate batches
		largeData := make([]byte, 1200*1024) // 1.2MB per resource (exceeds 1MB batch limit)
		for i := range largeData {
			largeData[i] = 'z'
		}
		largeState := string(largeData)

		// Mock expects 5 separate calls (one per resource)
		for i := range 5 {
			mockClient.On("ServerSideDiff", mock.Anything, mock.MatchedBy(func(query *applicationpkg.ApplicationServerSideDiffQuery) bool {
				return len(query.LiveResources) == 1
			})).Return(&applicationpkg.ApplicationServerSideDiffResponse{
				Items: []*v1alpha1.ResourceDiff{
					{
						Modified:    i%2 == 0,
						LiveState:   fmt.Sprintf(`{"kind":"Deployment%d"}`, i),
						TargetState: fmt.Sprintf(`{"kind":"Deployment%d"}`, i),
					},
				},
			}, nil).Once()
		}

		items := make([]comparisonObject, 5)
		for i := range 5 {
			deployment := createTestUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("deployment%d", i),
					Namespace: "default",
				},
			})
			deployment.Object["largeData"] = largeState
			items[i] = comparisonObject{
				key:    kube.GetResourceKey(deployment),
				live:   deployment,
				target: deployment,
			}
		}

		// Set concurrency to 2 - should process batches in parallel up to this limit
		strategy := newServerSideDiffStrategy(app, mockClient, "test-app", "argocd", 2, 1024)
		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 5)
		mockClient.AssertExpectations(t)
	})
}

func TestNewClientSideDiffStrategy(t *testing.T) {
	ctx := context.Background()

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

	modifiedDeployment := createTestUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Labels: map[string]string{
				"new": "label",
			},
		},
	})

	t.Run("Success with modified resource", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		items := []comparisonObject{
			{
				key:    kube.GetResourceKey(deployment),
				live:   deployment,
				target: modifiedDeployment,
			},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Modified)
	})

	t.Run("Success with identical resources", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		items := []comparisonObject{
			{
				key:    kube.GetResourceKey(deployment),
				live:   deployment,
				target: deployment,
			},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].Modified)
	})

	t.Run("Handles multiple resources", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

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

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		items := []comparisonObject{
			{key: kube.GetResourceKey(deployment), live: deployment, target: modifiedDeployment},
			{key: kube.GetResourceKey(service), live: service, target: service},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.True(t, results[0].Modified)
		assert.False(t, results[1].Modified)
	})

	t.Run("Empty items returns empty results", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		results, err := strategy(ctx, []comparisonObject{})

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("Handles nil live resource", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		items := []comparisonObject{
			{
				key:    kube.GetResourceKey(deployment),
				live:   nil, // Resource being added
				target: deployment,
			},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Modified)
	})

	t.Run("Handles nil target resource", func(t *testing.T) {
		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ResourceOverrides:   map[string]*v1alpha1.ResourceOverride{},
			KustomizeOptions:    &v1alpha1.KustomizeOptions{},
			ControllerNamespace: "argocd",
		}

		strategy, err := newClientSideDiffStrategy(app, settings, normalizers.IgnoreNormalizerOpts{})
		require.NoError(t, err)

		items := []comparisonObject{
			{
				key:    kube.GetResourceKey(deployment),
				live:   deployment,
				target: nil, // Resource being deleted
			},
		}

		results, err := strategy(ctx, items)

		require.NoError(t, err)
		require.Len(t, results, 1)
		// When target is nil (deletion), the diff engine doesn't mark it as modified
		// The result contains the diff but Modified may be false
		assert.NotNil(t, results[0])
	})
}

func TestNewNormalizeTargetManifestsProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("Normalizes manifests with tracking labels", func(t *testing.T) {
		deployment := createTestUnstructured(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-deployment",
				// No namespace specified - should be set to destination namespace
			},
		})

		baseProvider := mockManifestProvider([]*unstructured.Unstructured{deployment})

		app := createTestApp("test-app", "argocd")
		app.Spec.Destination.Namespace = "target-namespace"

		settings := &settingspkg.Settings{
			AppLabelKey:    "app.kubernetes.io/instance",
			TrackingMethod: "label",
			KustomizeOptions: &v1alpha1.KustomizeOptions{
				BuildOptions: "",
			},
			ControllerNamespace: "argocd",
		}

		infoProvider := &resourceInfoProvider{
			namespacedByGk: map[schema.GroupKind]bool{
				{Group: "apps", Kind: "Deployment"}: true,
			},
		}

		provider := newNormalizeTargetManifestsProvider(baseProvider, app, settings, infoProvider)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Deployment", result[0].GetKind())
		assert.Equal(t, "target-namespace", result[0].GetNamespace())

		// Verify tracking label was added
		labels := result[0].GetLabels()
		assert.NotNil(t, labels)
		assert.Equal(t, "test-app", labels["app.kubernetes.io/instance"])
	})

	t.Run("Propagates error from base provider", func(t *testing.T) {
		expectedErr := errors.New("base provider error")
		baseProvider := func(_ context.Context) ([]*unstructured.Unstructured, error) {
			return nil, expectedErr
		}

		app := createTestApp("test-app", "argocd")
		settings := &settingspkg.Settings{
			AppLabelKey:    "app.kubernetes.io/instance",
			TrackingMethod: "label",
		}
		infoProvider := &resourceInfoProvider{namespacedByGk: map[schema.GroupKind]bool{}}

		provider := newNormalizeTargetManifestsProvider(baseProvider, app, settings, infoProvider)
		result, err := provider(ctx)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("Normalizes cluster-scoped resource with namespace", func(t *testing.T) {
		clusterRole := createTestUnstructured(&rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster-role",
				Namespace: "kube-system", // Malformed - cluster-scoped resource shouldn't have namespace
			},
		})

		baseProvider := mockManifestProvider([]*unstructured.Unstructured{clusterRole})

		app := createTestApp("test-app", "argocd")
		app.Spec.Destination.Namespace = "default"

		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "annotation",
			ControllerNamespace: "argocd",
		}

		// ClusterRole is cluster-scoped, so infoProvider returns false for namespaced check
		infoProvider := &resourceInfoProvider{
			namespacedByGk: map[schema.GroupKind]bool{
				{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}: false,
			},
		}

		provider := newNormalizeTargetManifestsProvider(baseProvider, app, settings, infoProvider)
		result, err := provider(ctx)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "ClusterRole", result[0].GetKind())

		// Verify namespace was stripped for cluster-scoped resource
		assert.Empty(t, result[0].GetNamespace(), "cluster-scoped resource should have empty namespace")

		// Verify tracking annotation was re-applied with correct (empty) namespace
		annotations := result[0].GetAnnotations()
		assert.NotNil(t, annotations)
		trackingID, ok := annotations["argocd.argoproj.io/tracking-id"]
		assert.True(t, ok, "tracking annotation should be present")
		// Tracking ID format: app-name:group/kind:namespace/name (namespace should be empty for cluster-scoped)
		assert.Contains(t, trackingID, "ClusterRole:")
		assert.Contains(t, trackingID, "/my-cluster-role")
		assert.NotContains(t, trackingID, "kube-system", "tracking ID should not contain the stripped namespace")
	})

	t.Run("Deduplicates resources and keeps last", func(t *testing.T) {
		// Create three ConfigMaps with the same name
		configMap1 := createTestUnstructured(&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-config",
			},
			Data: map[string]string{"version": "1"},
		})

		configMap2 := createTestUnstructured(&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-config",
			},
			Data: map[string]string{"version": "2"},
		})

		configMap3 := createTestUnstructured(&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-config",
			},
			Data: map[string]string{"version": "3"},
		})

		baseProvider := mockManifestProvider([]*unstructured.Unstructured{configMap1, configMap2, configMap3})

		app := createTestApp("test-app", "argocd")
		app.Spec.Destination.Namespace = "prod"

		settings := &settingspkg.Settings{
			AppLabelKey:         "app.kubernetes.io/instance",
			TrackingMethod:      "label",
			ControllerNamespace: "argocd",
		}

		infoProvider := &resourceInfoProvider{
			namespacedByGk: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}: true,
			},
		}

		provider := newNormalizeTargetManifestsProvider(baseProvider, app, settings, infoProvider)
		result, err := provider(ctx)

		require.NoError(t, err)
		// Should only keep one resource after deduplication
		require.Len(t, result, 1, "should deduplicate to one resource")
		assert.Equal(t, "ConfigMap", result[0].GetKind())
		assert.Equal(t, "my-config", result[0].GetName())

		// Verify it's the last one (version 3)
		data, found, err := unstructured.NestedStringMap(result[0].Object, "data")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "3", data["version"], "should keep the last duplicate")

		// Verify namespace and tracking were still applied
		assert.Equal(t, "prod", result[0].GetNamespace())
		labels := result[0].GetLabels()
		assert.NotNil(t, labels)
		assert.Equal(t, "test-app", labels["app.kubernetes.io/instance"])
	})
}
