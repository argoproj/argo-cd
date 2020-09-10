package cache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis"
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
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 60*time.Second)
		err = client.Set(&Item{Key: "foo", Object: "bar"})
		assert.NoError(t, err)
	})

	t.Run("Successful get", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second)
		err = client.Get("foo", &res)
		assert.NoError(t, err)
		assert.Equal(t, res, "bar")
	})

	t.Run("Successful delete", func(t *testing.T) {
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second)
		err = client.Delete("foo")
		assert.NoError(t, err)
	})

	t.Run("Cache miss", func(t *testing.T) {
		var res string
		client := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 10*time.Second)
		err = client.Get("foo", &res)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache: key is missing")
	})
}
