package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	"github.com/stretchr/testify/mock"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestHandleModEvent_HasChanges(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything, mock.Anything).Return(nil).Once()
	clusterCache.On("EnsureSynced").Return(nil).Once()

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}, &appv1.Cluster{
		Server:     "https://mycluster",
		Config:     appv1.ClusterConfig{Username: "bar"},
		Namespaces: []string{"default"},
	})
}

func TestHandleModEvent_ClusterExcluded(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything, mock.Anything).Return(nil).Once()
	clusterCache.On("EnsureSynced").Return(nil).Once()

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
		clusterFilter: func(cluster *appv1.Cluster) bool {
			return false
		},
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}, &appv1.Cluster{
		Server:     "https://mycluster",
		Config:     appv1.ClusterConfig{Username: "bar"},
		Namespaces: []string{"default"},
	})

	assert.Len(t, clustersCache.clusters, 0)
}

func TestHandleModEvent_NoChanges(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything).Panic("should not invalidate")
	clusterCache.On("EnsureSynced").Return(nil).Panic("should not re-sync")

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	}, &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	})
}

func TestHandleAddEvent_ClusterExcluded(t *testing.T) {
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{},
		clusterFilter: func(cluster *appv1.Cluster) bool {
			return false
		},
	}
	clustersCache.handleAddEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	})

	assert.Len(t, clustersCache.clusters, 0)
}
