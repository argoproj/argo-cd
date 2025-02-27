package appstate

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
		1*time.Minute,
	)}
}

func TestCache_GetAppManagedResources(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	value := &[]*ResourceDiff{}
	err := cache.GetAppManagedResources("my-appname", value)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetAppManagedResources("my-appname", []*ResourceDiff{{Name: "my-name"}})
	require.NoError(t, err)
	// cache miss
	err = cache.GetAppManagedResources("other-appname", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetAppManagedResources("my-appname", value)
	require.NoError(t, err)
	assert.Equal(t, &[]*ResourceDiff{{Name: "my-name"}}, value)
}

func TestCache_GetAppResourcesTree(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	value := &ApplicationTree{}
	err := cache.GetAppResourcesTree("my-appname", value)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetAppResourcesTree("my-appname", &ApplicationTree{Nodes: []ResourceNode{{}}})
	require.NoError(t, err)
	// cache miss
	err = cache.GetAppResourcesTree("other-appname", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetAppResourcesTree("my-appname", value)
	require.NoError(t, err)
	assert.Equal(t, &ApplicationTree{Nodes: []ResourceNode{{}}}, value)
}

func TestCache_GetClusterInfo(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	res := &ClusterInfo{}
	err := cache.GetClusterInfo("http://minikube", res)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetClusterInfo("http://kind-cluster", &ClusterInfo{ServerVersion: "0.24.0"})
	require.NoError(t, err)
	// cache miss
	err = cache.GetClusterInfo("http://kind-clusterss", res)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetClusterInfo("http://kind-cluster", res)
	require.NoError(t, err)
	assert.Equal(t, &ClusterInfo{ServerVersion: "0.24.0"}, res)
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 1*time.Hour, cache.appStateCacheExpiration)
}
