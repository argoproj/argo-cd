package cache

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/oidc"
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
	_, err := cache.GetRepoConnectionState("my-repo")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetRepoConnectionState("my-repo", &ConnectionState{Status: "my-state"})
	assert.NoError(t, err)
	// cache miss
	_, err = cache.GetRepoConnectionState("other-repo")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetRepoConnectionState("my-repo")
	assert.NoError(t, err)
	assert.Equal(t, ConnectionState{Status: "my-state"}, value)
}

func TestCache_GetOIDCState(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetOIDCState("my-key")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetOIDCState("my-key", &oidc.OIDCState{ReturnURL: "my-return-url"})
	assert.NoError(t, err)
	//cache miss
	_, err = cache.GetOIDCState("other-key")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetOIDCState("my-key")
	assert.NoError(t, err)
	assert.Equal(t, &oidc.OIDCState{ReturnURL: "my-return-url"}, value)
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	assert.NoError(t, err)
	assert.Equal(t, 1*time.Hour, cache.connectionStatusCacheExpiration)
	assert.Equal(t, 3*time.Minute, cache.oidcCacheExpiration)
}
