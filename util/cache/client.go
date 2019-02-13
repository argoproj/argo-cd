package cache

import (
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache: key is missing")

type Item struct {
	Key    string
	Object interface{}
	// Expiration is the cache expiration time.
	Expiration time.Duration
}

type CacheClient interface {
	Set(item *Item) error
	Get(key string, obj interface{}) error
	Delete(key string) error
}
