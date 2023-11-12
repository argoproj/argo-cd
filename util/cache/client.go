package cache

import (
	"context"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache: key is missing")
var ErrCacheKeyLocked = errors.New("cache: key is locked")
var ErrLockWaitExpired = errors.New("cache: wait for lock exceeded max wait time")
var CacheLockedValue = "locked"

type Item struct {
	Key    string
	Object interface{}
	// Disable writing if key already exists (NX)
	DisableOverwrite bool
	// Expiration is the cache expiration time.
	Expiration time.Duration
}

type CacheClient interface {
	Set(item *Item) error
	Get(key string, obj interface{}) error
	Delete(key string) error
	OnUpdated(ctx context.Context, key string, callback func() error) error
	NotifyUpdated(key string) error
}
