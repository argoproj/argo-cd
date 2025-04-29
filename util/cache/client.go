package cache

import (
	"context"
	"errors"
	"time"
)

var (
	ErrCacheMiss      = errors.New("cache: key is missing")
	ErrCacheKeyLocked = errors.New("cache: key is locked")
	CacheLockedValue  = "locked"
)

type Item struct {
	Key             string
	Object          any
	CacheActionOpts CacheActionOpts
}

type CacheActionOpts struct {
	// Delete item from cache
	Delete bool
	// Disable writing if key already exists (NX)
	DisableOverwrite bool
	// Expiration is the cache expiration time.
	Expiration time.Duration
}

type CacheClient interface {
	Set(ctx context.Context, item *Item) error
	Rename(ctx context.Context, oldKey string, newKey string, expiration time.Duration) error
	Get(ctx context.Context, key string, obj any) error
	Delete(ctx context.Context, key string) error
	OnUpdated(ctx context.Context, key string, callback func() error) error
	NotifyUpdated(ctx context.Context, key string) error
}
