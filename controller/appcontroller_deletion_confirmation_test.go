package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v3/test"
)

// TestDeletionConfirmationWithEmptyCache reproduces the race condition where:
// 1. An app has resources with RequiresDeletionConfirmation=true in its status
// 2. The cache hasn't been populated yet (returns empty managedLiveObjs)
// 3. The deletion proceeds anyway instead of blocking for confirmation
func TestDeletionConfirmationWithEmptyCache(t *testing.T) {
	now := metav1.Now()
	defaultProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}

	t.Run("DeletionBlockedWithoutConfirmation_EmptyCache", func(t *testing.T) {
		app := newFakeApp()
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace

		// Simulate resources in status that require deletion confirmation
		// This is what gets persisted after reconciliation
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:                        "apps",
				Kind:                         "Deployment",
				Version:                      "v1",
				Name:                         "guestbook-ui",
				Namespace:                    test.FakeArgoCDNamespace,
				RequiresDeletionConfirmation: true, // This resource requires confirmation!
			},
			{
				Group:                        "",
				Kind:                         "Service",
				Version:                      "v1",
				Name:                         "guestbook-ui",
				Namespace:                    test.FakeArgoCDNamespace,
				RequiresDeletionConfirmation: false,
			},
		}

		// CRITICAL: Simulate the race condition - cache is empty (not populated yet)
		// Before the fix, this would cause the deletion to proceed incorrectly
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{}, // Empty cache!
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})

		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)

		// Fixed: Now we check app.Status.Resources before getting live objects from cache
		// Deletion is properly blocked even when cache is empty
		assert.False(t, patched, "App should NOT be deleted without confirmation, even with empty cache")
	})

	t.Run("DeletionBlockedWithoutConfirmation_MultipleResources", func(t *testing.T) {
		app := newFakeApp()
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace

		// Simulate resources in status that require deletion confirmation
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:                        "apps",
				Kind:                         "Deployment",
				Version:                      "v1",
				Name:                         "guestbook-ui",
				Namespace:                    test.FakeArgoCDNamespace,
				RequiresDeletionConfirmation: true, // This resource requires confirmation!
			},
		}

		// Simulate empty cache (the race condition scenario)
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{}, // Empty cache!
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})

		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)

		// Deletion should be blocked because we check app.Status.Resources
		// before trying to get live objects from cache
		assert.False(t, patched, "App should NOT be deleted without confirmation")
	})

	t.Run("DeletionProceedsWithConfirmation", func(t *testing.T) {
		app := newFakeApp()
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace

		// Add the deletion approval annotation
		deletionTime := app.DeletionTimestamp.Time
		// Set confirmation timestamp AFTER deletion timestamp
		confirmationTime := deletionTime.Add(1 * time.Second)
		app.Annotations = map[string]string{
			"argocd.argoproj.io/deletion-approved": confirmationTime.Format(time.RFC3339),
		}

		// Resources requiring confirmation
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:                        "apps",
				Kind:                         "Deployment",
				Version:                      "v1",
				Name:                         "guestbook-ui",
				Namespace:                    test.FakeArgoCDNamespace,
				RequiresDeletionConfirmation: true,
			},
		}

		// Empty cache
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})

		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)

		// With confirmation, deletion should proceed
		assert.True(t, patched, "With confirmation, app should be deleted")
	})

	t.Run("DeletionProceedsWhenNoResourcesRequireConfirmation", func(t *testing.T) {
		app := newFakeApp()
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace

		// Resources that DON'T require confirmation
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:                        "apps",
				Kind:                         "Deployment",
				Version:                      "v1",
				Name:                         "guestbook-ui",
				Namespace:                    test.FakeArgoCDNamespace,
				RequiresDeletionConfirmation: false, // No confirmation needed
			},
		}

		// Empty cache
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})

		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)

		// No confirmation needed, deletion should proceed
		assert.True(t, patched, "When no resources require confirmation, app should be deleted")
	})
}
