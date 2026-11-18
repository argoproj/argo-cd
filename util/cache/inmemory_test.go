package cache

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type foo struct {
	Bar string
}

func TestInMemoryCache(t *testing.T) {
	t.Parallel()
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

func TestInMemoryCacheOnUpdated(t *testing.T) {
	t.Parallel()
	cache := NewInMemoryCache(1 * time.Hour)

	calls := 0
	err := cache.OnUpdated(t.Context(), "my-key", func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "the callback reads the current value once; there are no updates to wait for")

	expected := errors.New("callback failed")
	err = cache.OnUpdated(t.Context(), "my-key", func() error { return expected })
	require.ErrorIs(t, err, expected)
}
