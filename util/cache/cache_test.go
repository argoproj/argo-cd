package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/common"
)

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.client.(*redisCache).expiration)
}

func NewInMemoryRedis() (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr.Close
}

func TestCacheClient(t *testing.T) {
	clientRedis, stopRedis := NewInMemoryRedis()
	defer stopRedis()
	redisCache := NewRedisCache(clientRedis, 5*time.Second, RedisCompressionNone)
	clientMemCache := NewInMemoryCache(60 * time.Second)
	twoLevelClient := NewTwoLevelClient(redisCache, 5*time.Second)
	// Run tests for both Redis and InMemoryCache
	for _, client := range []CacheClient{clientMemCache, redisCache, twoLevelClient} {
		cache := NewCache(client)
		t.Run("SetItem", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			var output string
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
		})
		t.Run("SetCacheItem W/Disable Overwrite", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			var output string
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
			err = cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output, "output should not have changed with DisableOverwrite set to true")
		})
		t.Run("GetItem", func(t *testing.T) {
			var val string
			err := cache.GetItem("foo", &val)
			require.NoError(t, err)
			assert.Equal(t, "bar", val)
		})
		t.Run("DeleteItem", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 0, Delete: true})
			require.NoError(t, err)
			var val string
			err = cache.GetItem("foo", &val)
			require.Error(t, err)
			assert.Empty(t, val)
		})
		t.Run("Check for nil items", func(t *testing.T) {
			err := cache.SetItem("foo", nil, &CacheActionOpts{Expiration: 0, Delete: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot set nil item")
			err = cache.GetItem("foo", nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot get item")
		})
	}
}

// Smoke test to ensure key changes aren't done accidentally
func TestGenerateCacheKey(t *testing.T) {
	client := NewInMemoryCache(60 * time.Second)
	cache := NewCache(client)
	testKey := cache.generateFullKey("testkey")
	assert.Equal(t, fmt.Sprintf("testkey|%s", common.CacheVersion), testKey)
}
