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
	ctx := t.Context()
	cache := NewInMemoryCache(1 * time.Hour)
	// https://stackoverflow.com/questions/46671636/gob-decode-giving-decodevalue-of-unassignable-value-error
	obj := &foo{}
	// cache miss
	err := cache.Get(ctx, "my-key", obj)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.Set(ctx, &Item{Key: "my-key", Object: &foo{Bar: "bar"}})
	require.NoError(t, err)
	err = cache.Get(ctx, "my-key", obj)
	require.NoError(t, err)
	assert.Equal(t, &foo{Bar: "bar"}, obj)
}
