package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"
	argosettings "github.com/argoproj/argo-cd/v3/util/settings"
)

// TestHandleModEvent_HardRefreshWithTaints tests that hard refresh with taints
// properly repopulates the cache synchronously to avoid missing resources
func TestHandleModEvent_HardRefreshWithTaints(t *testing.T) {
	now := metav1.NewTime(time.Now())
	tenSecondsAgo := metav1.NewTime(time.Now().Add(-10 * time.Second))

	testCases := []struct {
		name                     string
		hasTaintsBeforeRefresh   bool
		hasTaintsAfterValidation bool
		expectSyncEnsureSynced   bool
		expectAsyncEnsureSynced  bool
	}{
		{
			name:                     "hard refresh with taints cleared - should sync synchronously",
			hasTaintsBeforeRefresh:   true,
			hasTaintsAfterValidation: false,
			expectSyncEnsureSynced:   true,
			expectAsyncEnsureSynced:  false,
		},
		{
			name:                     "hard refresh with taints remaining - should sync synchronously",
			hasTaintsBeforeRefresh:   true,
			hasTaintsAfterValidation: true,
			expectSyncEnsureSynced:   true,
			expectAsyncEnsureSynced:  false,
		},
		{
			name:                     "hard refresh without taints - should sync asynchronously",
			hasTaintsBeforeRefresh:   false,
			hasTaintsAfterValidation: false,
			expectSyncEnsureSynced:   false,
			expectAsyncEnsureSynced:  true,
		},
		{
			name:                     "no hard refresh with taints - no sync at all",
			hasTaintsBeforeRefresh:   true,
			hasTaintsAfterValidation: true,
			expectSyncEnsureSynced:   false,
			expectAsyncEnsureSynced:  false, // No refresh at all
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			clusterCache := &mocks.ClusterCache{}
			clusterInfo := cache.ClusterInfo{
				LastCacheSyncTime: &tenSecondsAgo.Time,
			}
			// Only expect GetClusterInfo if we're doing a hard refresh
			if tc.name != "no hard refresh with taints - no sync at all" {
				clusterCache.On("GetClusterInfo").Return(clusterInfo)
			}

			// Setup database and settings
			db := &dbmocks.ArgoDB{}
			db.On("GetApplicationControllerReplicas").Return(1)
			fakeClient := fake.NewClientset()
			settingsMgr := argosettings.NewSettingsManager(t.Context(), fakeClient, "argocd")

			// Create cache instance
			clustersCache := &liveStateCache{
				clusters: map[string]cache.ClusterCache{
					"https://mycluster": clusterCache,
				},
				clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
				settingsMgr:     settingsMgr,
				taintManager:    newClusterTaintManager(),
			}

			// Setup expectations for Invalidate
			if tc.name != "no hard refresh with taints - no sync at all" {
				clusterCache.On("Invalidate", mock.Anything).Return(nil).Once()

				// Mock FindResources for validateAndClearHealthyTaints when there are taints
				if tc.hasTaintsBeforeRefresh {
					if tc.hasTaintsAfterValidation {
						// Validation fails - FindResources panics or returns error
						clusterCache.On("FindResources", "", mock.Anything).Panic("conversion webhook error").Once()
					} else {
						// Validation succeeds - FindResources returns normally with valid resources
						mockResource := &cache.Resource{
							Ref: corev1.ObjectReference{
								APIVersion: "example.io/v1",
								Kind:       "Example",
								Name:       "test-example",
								Namespace:  "default",
							},
							ResourceVersion: "123",
						}
						clusterCache.On("FindResources", "", mock.Anything).Return(map[kube.ResourceKey]*cache.Resource{
							kube.NewResourceKey("example.io", "Example", "default", "test-example"): mockResource,
						}).Once()
					}
				}
			}

			// Setup expectations for EnsureSynced
			if tc.expectSyncEnsureSynced {
				// Synchronous call - will be called directly
				if tc.hasTaintsAfterValidation {
					// When taints remain, EnsureSynced returns an error but still populates non-tainted resources
					clusterCache.On("EnsureSynced").Return(assert.AnError).Once()
				} else {
					// When all taints cleared, EnsureSynced succeeds
					clusterCache.On("EnsureSynced").Return(nil).Once()
				}
			}

			if tc.expectAsyncEnsureSynced {
				// Async call - will be called in a goroutine
				// We can't easily test goroutine execution, but we can verify it wasn't called synchronously
				clusterCache.On("EnsureSynced").Return(nil).Maybe()
			}

			// Set up initial taint state
			if tc.hasTaintsBeforeRefresh {
				clustersCache.MarkClusterTainted("https://mycluster", "conversion webhook error", "example.io/v1, Kind=Example", "conversion_webhook")
			}

			// Create old and new cluster states
			oldCluster := &appv1.Cluster{
				Server: "https://mycluster",
				Config: appv1.ClusterConfig{Username: "foo"},
			}

			newCluster := &appv1.Cluster{
				Server: "https://mycluster",
				Config: appv1.ClusterConfig{Username: "foo"},
			}

			// Set RefreshRequestedAt for hard refresh test cases
			if tc.name != "no hard refresh with taints - no sync at all" {
				newCluster.RefreshRequestedAt = &now
			}

			// Execute the method under test
			clustersCache.handleModEvent(oldCluster, newCluster)

			// Give async operations a moment to start (if any)
			time.Sleep(10 * time.Millisecond)

			// Verify expectations
			clusterCache.AssertExpectations(t)

			// Additional assertions
			shouldClearTaint := tc.hasTaintsBeforeRefresh && !tc.hasTaintsAfterValidation && tc.name != "no hard refresh with taints - no sync at all"
			if shouldClearTaint {
				assert.False(t, clustersCache.IsClusterTainted("https://mycluster"), "Cluster should not be tainted after successful validation")
			}

			if tc.hasTaintsBeforeRefresh && tc.hasTaintsAfterValidation {
				assert.True(t, clustersCache.IsClusterTainted("https://mycluster"), "Cluster should still be tainted when validation fails")
			}
		})
	}
}

// TestHandleModEvent_ResourcesAvailableAfterHardRefresh verifies that resources
// become available immediately after hard refresh with synchronous EnsureSynced
func TestHandleModEvent_ResourcesAvailableAfterHardRefresh(t *testing.T) {
	now := metav1.NewTime(time.Now())
	tenSecondsAgo := metav1.NewTime(time.Now().Add(-10 * time.Second))

	// Setup mock cluster cache
	clusterCache := &mocks.ClusterCache{}
	clusterInfo := cache.ClusterInfo{
		LastCacheSyncTime: &tenSecondsAgo.Time,
	}
	clusterCache.On("GetClusterInfo").Return(clusterInfo)

	// Track whether EnsureSynced was called and when
	ensureSyncedCalled := false
	resourcesAvailable := false

	// Setup Invalidate expectation
	clusterCache.On("Invalidate", mock.Anything).Run(func(_ mock.Arguments) {
		// After invalidate, resources are cleared
		resourcesAvailable = false
	}).Return(nil).Once()

	// Setup EnsureSynced to mark resources as available
	clusterCache.On("EnsureSynced").Run(func(_ mock.Arguments) {
		ensureSyncedCalled = true
		// Simulate resources being populated by EnsureSynced
		resourcesAvailable = true
	}).Return(nil).Once()

	// Setup database and settings
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	fakeClient := fake.NewClientset()
	settingsMgr := argosettings.NewSettingsManager(t.Context(), fakeClient, "argocd")

	// Create cache instance with tainted cluster
	clustersCache := &liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
		clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		settingsMgr:     settingsMgr,
		taintManager:    newClusterTaintManager(),
	}

	// Mock FindResources for validateAndClearHealthyTaints - simulate successful validation
	clusterCache.On("FindResources", "", mock.Anything).Run(func(_ mock.Arguments) {
		// The validation will succeed, clearing the taint
	}).Return(map[kube.ResourceKey]*cache.Resource{}).Once()

	// Mark cluster as tainted
	clustersCache.MarkClusterTainted("https://mycluster", "conversion webhook error", "example.io/v1, Kind=Example", "conversion_webhook")

	// Create clusters for hard refresh
	oldCluster := &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}

	newCluster := &appv1.Cluster{
		Server:             "https://mycluster",
		Config:             appv1.ClusterConfig{Username: "foo"},
		RefreshRequestedAt: &now, // Trigger hard refresh
	}

	// Execute the method under test
	clustersCache.handleModEvent(oldCluster, newCluster)

	// Verify that EnsureSynced was called synchronously (not in a goroutine)
	assert.True(t, ensureSyncedCalled, "EnsureSynced should have been called synchronously")
	assert.True(t, resourcesAvailable, "Resources should be available immediately after handleModEvent returns")

	// Verify all expectations were met
	clusterCache.AssertExpectations(t)
}
