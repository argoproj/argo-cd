package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type foo struct {
	Bar string
}

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	// https://stackoverflow.com/questions/46671636/gob-decode-giving-decodevalue-of-unassignable-value-error
	obj := &foo{}
	// cache miss
	err := cache.Get("my-key", obj)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.Set(&Item{Key: "my-key", Object: &foo{Bar: "bar"}})
	require.NoError(t, err)
	err = cache.Get("my-key", obj)
	require.NoError(t, err)
	assert.Equal(t, &foo{Bar: "bar"}, obj)
}

func TestInMemoryCacheDeleteByPattern(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	obj := &foo{}
	// cache miss
	err := cache.Get("my-key:1", obj)
	assert.Equal(t, ErrCacheMiss, err)
	err = cache.Get("my-key:2", obj)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.Set(&Item{Key: "my-key:1", Object: &foo{Bar: "bar"}})
	require.NoError(t, err)
	err = cache.Set(&Item{Key: "my-key:2", Object: &foo{Bar: "bar"}})
	require.NoError(t, err)
	err = cache.Set(&Item{Key: "other-key:1", Object: &foo{Bar: "bar"}})
	require.NoError(t, err)
	err = cache.Get("my-key:1", obj)
	require.NoError(t, err)
	err = cache.Get("my-key:2", obj)
	require.NoError(t, err)
	err = cache.Get("other-key:1", obj)
	require.NoError(t, err)
	// clear cache by pattern
	err = cache.DeleteByPattern("my-key:")
	require.NoError(t, err)
	// cache miss after deletion
	err = cache.Get("my-key:1", obj)
	assert.Equal(t, ErrCacheMiss, err)
	err = cache.Get("my-key:2", obj)
	assert.Equal(t, ErrCacheMiss, err)
	// other key should not be deleted
	err = cache.Get("other-key:1", obj)
	require.NoError(t, err)
}
