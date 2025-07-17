package cache

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		),
		1*time.Minute,
		1*time.Minute,
		1*time.Minute,
	)}
}

func TestCache_GetRepoConnectionState(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetRepoConnectionState("my-repo", "")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetRepoConnectionState("my-repo", "", &ConnectionState{Status: "my-state"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.GetRepoConnectionState("my-repo", "some-project")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetRepoConnectionState("my-repo", "some-project", &ConnectionState{Status: "my-project-state"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.GetRepoConnectionState("other-repo", "")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetRepoConnectionState("my-repo", "")
	require.NoError(t, err)
	assert.Equal(t, ConnectionState{Status: "my-state"}, value)
	// cache hit
	value, err = cache.GetRepoConnectionState("my-repo", "some-project")
	require.NoError(t, err)
	assert.Equal(t, ConnectionState{Status: "my-project-state"}, value)
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 1*time.Hour, cache.connectionStatusCacheExpiration)
	assert.Equal(t, 3*time.Minute, cache.oidcCacheExpiration)
}
