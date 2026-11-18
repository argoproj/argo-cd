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
	Set(item *Item) error
	Rename(oldKey string, newKey string, expiration time.Duration) error
	Get(key string, obj any) error
	Delete(key string) error
	// OnUpdated blocks until ctx is done, calling callback once it listens for updates of key and then after
	// each update. The first call is how a caller reads the current value without missing an update that is
	// published while it does. An error from callback ends the wait and is returned.
	OnUpdated(ctx context.Context, key string, callback func() error) error
	NotifyUpdated(key string) error
}
