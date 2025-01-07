package cache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"

	"github.com/redis/go-redis/v9"
)

func Test_ReconnectCallbackHookCalled(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()

	called := false
	hook := NewArgoRedisHook(func() {
		called = true
	})

	faultyDNSRedisClient := redis.NewClient(&redis.Options{Addr: "invalidredishost.invalid:12345"})
	faultyDNSRedisClient.AddHook(hook)

	faultyDNSClient := NewRedisCache(faultyDNSRedisClient, 60*time.Second, RedisCompressionNone)
	err = faultyDNSClient.Set(&Item{Key: "baz", Object: "foo"})
	assert.Equal(t, called, true)
	assert.Error(t, err)
}

func Test_ReconnectCallbackHookNotCalled(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()

	called := false
	hook := NewArgoRedisHook(func() {
		called = true
	})

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisClient.AddHook(hook)
	client := NewRedisCache(redisClient, 60*time.Second, RedisCompressionNone)

	err = client.Set(&Item{Key: "foo", Object: "bar"})
	assert.Equal(t, called, false)
	assert.NoError(t, err)
}
