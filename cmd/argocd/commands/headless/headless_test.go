package headless

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/cache"
)

func TestDoLazy_ExternalRedisConnection(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	// Set ARGOCD_REDIS_SERVER to simulate external Redis (miniredis)
	t.Setenv("ARGOCD_REDIS_SERVER", mr.Addr())

	client := &forwardCacheClient{
		compression: cache.RedisCompressionNone,
	}

	err = client.doLazy(func(c cache.CacheClient) error {
		assert.NotNil(t, c)
		return nil
	})

	require.NoError(t, err)
	assert.NotNil(t, client.client)
}

func TestDoLazy_FallbackPath(t *testing.T) {
	// Just in case
	t.Setenv("ARGOCD_REDIS_SERVER", "")

	client := &forwardCacheClient{
		context: "invalid-context",
	}

	err := client.doLazy(func(_ cache.CacheClient) error {
		return nil
	})

	// Verify failure in finding Kubernetes context, confirming the fallback logic attempted to use in cluster Redis discovery.
	require.Error(t, err)
}

func TestDoLazy_CacheOperations(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	t.Setenv("ARGOCD_REDIS_SERVER", mr.Addr())

	client := &forwardCacheClient{}

	err = client.doLazy(func(cacheClient cache.CacheClient) error {
		testKey := "argocd-test"
		testVal := "hello-argo"

		// Verify basic cache operations with the external Redis client
		err := cacheClient.Set(&cache.Item{Key: testKey, Object: testVal})
		require.NoError(t, err)

		var result string
		err = cacheClient.Get(testKey, &result)
		require.NoError(t, err)
		assert.Equal(t, testVal, result)

		err = cacheClient.Delete(testKey)
		require.NoError(t, err)

		err = cacheClient.Get(testKey, &result)
		require.Error(t, err)

		return nil
	})

	require.NoError(t, err)
}
