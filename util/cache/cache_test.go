package cache

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	assert.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.client.(*redisCache).expiration)
}

func TestCacheClient(t *testing.T) {
	client := NewInMemoryCache(60 * time.Second)
	cache := NewCache(client)
	t.Run("SetItem", func(t *testing.T) {
		err := cache.SetItem("foo", "bar", 60*time.Second, false)
		assert.NoError(t, err)
	})
	t.Run("GetItem", func(t *testing.T) {
		var val string
		err := cache.GetItem("foo", &val)
		assert.NoError(t, err)
		assert.Equal(t, "bar", val)
	})
	t.Run("DeleteItem", func(t *testing.T) {
		err := cache.SetItem("foo", "bar", 0, true)
		assert.NoError(t, err)
		var val string
		err = cache.GetItem("foo", &val)
		assert.Error(t, err)
		assert.Empty(t, val)
	})
	t.Run("Check for nil items", func(t *testing.T) {
		err := cache.SetItem("foo", nil, 0, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set item")
		err = cache.GetItem("foo", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot get item")
	})
}
