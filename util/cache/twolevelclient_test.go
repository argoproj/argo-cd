package cache

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingCacheClient wraps a CacheClient, counts the Set calls that reach it and can make calls fail.
type countingCacheClient struct {
	CacheClient
	sets   int
	getErr error
	setErr error
}

func (c *countingCacheClient) Set(item *Item) error {
	c.sets++
	if c.setErr != nil {
		return c.setErr
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

	// The marker is written when the client is created and the key on the first write. The first unchanged write
	// finds the marker, so both unchanged writes are skipped.
	assert.Equal(t, 2, external.sets)
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

	// A failed check must not be read as data loss, so the unchanged value is not written again. The two writes are
	// the marker at start-up and the key.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.Equal(t, 2, external.sets)

	var value string
	require.NoError(t, client.Get("key", &value))
	assert.Equal(t, "value", value)
}

// newTestLogHook captures the standard logger's entries for the duration of the test.
func newTestLogHook(t *testing.T) *logtest.Hook {
	t.Helper()
	logger := log.StandardLogger()
	hooks := logger.ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { logger.ReplaceHooks(hooks) })
	return logtest.NewLocal(logger)
}

// missingMarkerInfoLogs counts the info-level entries that report the marker as missing.
func missingMarkerInfoLogs(hook *logtest.Hook) int {
	count := 0
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.InfoLevel && strings.Contains(entry.Message, "is missing from external cache") {
			count++
		}
	}
	return count
}

func TestTwoLevelClient_MissingMarkerIsLoggedOnlyOnDataLoss(t *testing.T) {
	hook := newTestLogHook(t)
	mr, redisCache := newTestRedisCache(t)
	client := NewTwoLevelClient(redisCache, time.Minute)
	client.externalCacheCheckInterval = 0

	// First check after start-up: the marker was written when the client was created, so the check finds it.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.Equal(t, 0, missingMarkerInfoLogs(hook))

	// The marker expires. miniredis only moves its own clock forward, so move the write time back as well.
	mr.FastForward(externalCacheMarkerExpiration + time.Minute)
	client.externalCacheMarkerWrittenAt.Store(time.Now().Add(-externalCacheMarkerExpiration - time.Minute).UnixNano())
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.Equal(t, 0, missingMarkerInfoLogs(hook))

	// Redis loses its data.
	mr.FlushAll()
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.Equal(t, 1, missingMarkerInfoLogs(hook))
}

func TestTwoLevelClient_MarkerWriteErrorAtStartUpFallsBackToCheck(t *testing.T) {
	hook := newTestLogHook(t)
	_, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache, setErr: errors.New("write failed")}

	// The external cache rejects the marker at start-up, so no write time is recorded.
	client := NewTwoLevelClient(external, time.Minute)
	client.externalCacheCheckInterval = 0
	assert.Zero(t, client.externalCacheMarkerWrittenAt.Load())

	// Once the external cache accepts writes, the first check finds no marker, flushes and writes it. The marker was
	// never written, so this is not logged as data loss.
	external.setErr = nil
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	assert.NotZero(t, client.externalCacheMarkerWrittenAt.Load())
	assert.Equal(t, 0, missingMarkerInfoLogs(hook))
}

func TestTwoLevelClient_SetRetriesValueAfterExternalWriteError(t *testing.T) {
	_, redisCache := newTestRedisCache(t)
	external := &countingCacheClient{CacheClient: redisCache}
	client := NewTwoLevelClient(external, time.Minute)
	client.externalCacheCheckInterval = time.Hour

	external.setErr = errors.New("write failed")
	require.Error(t, client.Set(&Item{Key: "key", Object: "value"}))
	external.setErr = nil

	// The retry with the same value must reach the external cache.
	require.NoError(t, client.Set(&Item{Key: "key", Object: "value"}))
	var value string
	require.NoError(t, redisCache.Get("key", &value))
	assert.Equal(t, "value", value)
}

// Two controllers share one external cache (Redis) and each has its own in-memory layer.
// When another controller overwrites a key, a later Set of the value this controller wrote
// earlier must still reach the external cache.
func TestTwoLevelClient_SetAfterKeyOverwrittenByAnotherClient(t *testing.T) {
	t.Parallel()
	external := NewInMemoryCache(time.Hour) // stands in for Redis
	controllerA := NewTwoLevelClient(external, time.Hour)
	controllerB := NewTwoLevelClient(external, time.Hour)
	key := "app|resources-tree|my-app"

	// App is on cluster B: B writes its tree.
	require.NoError(t, controllerB.Set(&Item{Key: key, Object: &foo{Bar: "healthy"}}))
	// App moves to cluster A: A overwrites the key with its tree.
	require.NoError(t, controllerA.Set(&Item{Key: key, Object: &foo{Bar: "suspended"}}))
	// App moves back to B: B drops its local copy because another controller may have written the key.
	controllerB.InvalidateLocal(key)
	// B computes the same tree it wrote before.
	require.NoError(t, controllerB.Set(&Item{Key: key, Object: &foo{Bar: "healthy"}}))

	got := &foo{}
	require.NoError(t, external.Get(key, got))
	assert.Equal(t, "healthy", got.Bar)
}
