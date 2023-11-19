package cache

import (
	"context"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache: key is missing")
var ErrCacheKeyLocked = errors.New("cache: key is locked")
var CacheLockedValue = "locked"

type CacheType int

const (
	CacheTypeDefault CacheType = iota
	CacheTypeInMemory
	CacheTypeTwoLevel
	CacheTypeExternal
)

type CacheActionOpts struct {
	// Delete item from cache
	Delete bool
	// Disable writing if key already exists (NX)
	DisableOverwrite bool
	// Expiration is the cache expiration time.
	Expiration time.Duration
	// Determines the cache type to use in case of two level cache, Default == Two Level(in-memory + external)
	CacheType CacheType
}

type Item struct {
	Key    string
	Object interface{}
	CacheActionOpts
}

type CacheClient interface {
	Set(item *Item) error
	Get(item *Item) error
	Delete(key string) error
	OnUpdated(ctx context.Context, key string, callback func() error) error
	NotifyUpdated(key string) error
}
