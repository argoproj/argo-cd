package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestRedisSetCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	assert.NotNil(t, mr)

	t.Run("Successful set", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 60*time.Second, RedisCompressionNone)
		err = client.Set(&Item{Key: "foo", Object: "bar"})
		assert.NoError(t, err)
	})

	t.Run("Successful get", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		assert.NoError(t, err)
		assert.Equal(t, res, "bar")
	})

	t.Run("Successful delete", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Delete("foo")
		assert.NoError(t, err)
	})

	t.Run("Cache miss", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second, RedisCompressionNone)
		err = client.Get("foo", &res)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache: key is missing")
	})
}

func TestRedisSetCacheCompressed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()
	assert.NotNil(t, mr)

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	client := NewRedisCache(redisClient, 10*time.Second, RedisCompressionGZip)
	testValue := "my-value"
	assert.NoError(t, client.Set(&Item{Key: "my-key", Object: testValue}))

	compressedData, err := redisClient.Get(context.Background(), "my-key.gz").Bytes()
	assert.NoError(t, err)
	assert.True(t, len(compressedData) > len([]byte(testValue)), "compressed data is bigger than uncompressed")

	var result string
	assert.NoError(t, client.Get("my-key", &result))

	assert.Equal(t, testValue, result)
}
