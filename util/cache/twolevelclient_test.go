package cache

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingCacheClient wraps a CacheClient, counts the Set calls that reach it and can make calls fail.
type countingCacheClient struct {
	CacheClient
	sets            int
	getErr          error
	setErrKeyPrefix string
}

func (c *countingCacheClient) Set(item *Item) error {
	c.sets++
	if c.setErrKeyPrefix != "" && strings.HasPrefix(item.Key, c.setErrKeyPrefix) {
		return errors.New("write failed")
	}
	return c.CacheClient.Set(item)
}

func (c *countingCacheClient) Get(key string, obj any) error {
	if c.getErr != nil {
		return c.getErr
	}
	return c.CacheClient.Get(key, obj)
}

func newTestRedisCache(t *testing.T) (*miniredis.Miniredis, CacheClient) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, NewRedisCache(client, time.Hour, RedisCompressionNone)
}

func TestTwoLevelClient_SetSkipsUnchangedValue(t *testing.T) {
	_, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache}
	client := NewTwoLevelClient(external, time.Minute)

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	// The first write stores the key. The first unchanged write finds no marker yet, so it stores the marker and
	// writes the key once more. The last write is skipped.
	assert.Equal(t, 3, external.sets)
}

func TestTwoLevelClient_SetWritesChangedValue(t *testing.T) {
	_, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache}
	client := NewTwoLevelClient(external, time.Minute)

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value1"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value2"}))

	var value string
	require.NoError(t, redisCache.Get("key", &value))
	assert.Equal(t, "value2", value)
}

func TestTwoLevelClient_SetRewritesKeyAfterExternalCacheIsFlushed(t *testing.T) {
	mr, redisCache := newTestRedisCache(t)
	client := NewTwoLevelClient(redisCache, time.Minute)
	client.externalCacheCheckInterval = 0

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	// The first unchanged write runs the liveness check and stores the marker.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	// Simulate a Redis restart without persistence.
	mr.FlushAll()

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	var value string
	require.NoError(t, redisCache.Get("key", &value))
	assert.Equal(t, "value", value)
}

func TestTwoLevelClient_SetRewritesAllKeysAfterExternalCacheIsFlushed(t *testing.T) {
	mr, redisCache := newTestRedisCache(t)
	client := NewTwoLevelClient(redisCache, time.Minute)
	client.externalCacheCheckInterval = 0

	keys := []string{"app|managed-resources|a", "app|managed-resources|b", "app|resources-tree|a"}
	for _, key := range keys {
		require.NoError(t, client.Set(&Item{Key: key, Object: "value"}))
	}
	require.NoError(t, client.Set(&Item{Key: keys[0], Object: "value"}))

	mr.FlushAll()

	// Only the first Set detects the flush; the rest must still reach Redis.
	for _, key := range keys {
		require.NoError(t, client.Set(&Item{Key: key, Object: "value"}))
	}

	for _, key := range keys {
		var value string
		require.NoError(t, redisCache.Get(key, &value), key)
		assert.Equal(t, "value", value)
	}
}

func TestTwoLevelClient_ExternalCacheCheckIsRateLimited(t *testing.T) {
	mr, redisCache := newTestRedisCache(t)
	client := NewTwoLevelClient(redisCache, time.Minute)
	client.externalCacheCheckInterval = time.Hour

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	mr.FlushAll()

	// The next check is not due yet, so the unchanged value is not written.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	var value string
	require.ErrorIs(t, redisCache.Get("key", &value), ErrCacheMiss)

	// Once the interval has elapsed, the flush is detected and the key is rewritten.
	client.lastExternalCacheCheck.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, redisCache.Get("key", &value))
	assert.Equal(t, "value", value)
}

func TestTwoLevelClient_ExternalCacheCheckErrorKeepsInMemoryCache(t *testing.T) {
	_, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache}
	client := NewTwoLevelClient(external, time.Minute)
	client.externalCacheCheckInterval = 0

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	external.getErr = errors.New("connection refused")

	// A failed check must not be read as data loss, so the unchanged value is not written again.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.Equal(t, 1, external.sets)

	var value string
	require.NoError(t, client.Get("key", &value))
	assert.Equal(t, "value", value)
}

func TestTwoLevelClient_ExternalCacheMarkerIsRefreshed(t *testing.T) {
	mr, redisCache := newTestRedisCache(t)
	client := NewTwoLevelClient(redisCache, time.Minute)
	client.externalCacheCheckInterval = 0

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	// The first unchanged write runs the check and stores the marker.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.True(t, mr.Exists(client.externalCacheMarkerKey))

	// A check shortly before the marker expires finds it and writes it again.
	mr.FastForward(externalCacheMarkerExpiration - time.Hour)
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	mr.FastForward(2 * time.Hour)
	assert.True(t, mr.Exists(client.externalCacheMarkerKey))
}

func TestTwoLevelClient_MarkerWriteErrorKeepsWritingToExternalCache(t *testing.T) {
	mr, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache, setErrKeyPrefix: "twolevelclient|marker|"}
	client := NewTwoLevelClient(external, time.Minute)
	client.externalCacheCheckInterval = 0

	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	// Remove only the key. If the marker had been written, the next check would find it and skip the write.
	// Since it could not be written, the check finds it missing again and the unchanged value is written.
	mr.Del("key")
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))

	var value string
	require.NoError(t, redisCache.Get("key", &value))
	assert.Equal(t, "value", value)
}
