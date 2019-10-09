package controllercache

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
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
	_, err := cache.GetAppManagedResources("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetAppManagedResources("my-appname", []*ResourceDiff{{Name: "my-name"}})
	assert.NoError(t, err)
	// cache miss
	_, err = cache.GetAppManagedResources("other-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetAppManagedResources("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, []*ResourceDiff{{Name: "my-name"}}, value)
}

func TestCache_GetAppResourcesTree(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetAppResourcesTree("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetAppResourcesTree("my-appname", &ApplicationTree{Nodes: []ResourceNode{{}}})
	assert.NoError(t, err)
	// cache miss
	_, err = cache.GetAppResourcesTree("other-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetAppResourcesTree("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, &ApplicationTree{Nodes: []ResourceNode{{}}}, value)
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	assert.NoError(t, err)
	assert.Equal(t, 1*time.Hour, cache.appStateCacheExpiration)
}
