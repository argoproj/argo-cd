// Package hostscan memoizes a per-cluster snapshot so application-controller
// reconciles do not each walk the whole live-state cache.
package hostscan

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Cache stores loaded values for a TTL. A failed load is not cached.
// The zero Cache is ready to use, with a 2s TTL.
type Cache[T any] struct {
	// TTL is how long a successful value is reused. Zero means 2s.
	TTL time.Duration

	mu    sync.Mutex
	items map[string]entry[T]
	group singleflight.Group
}

type entry[T any] struct {
	value T
	at    time.Time
}

// Get returns the cached value for key, or calls load once for concurrent callers.
func (c *Cache[T]) Get(key string, load func() (T, error)) (T, error) {
	if v, ok := c.fresh(key); ok {
		return v, nil
	}
	got, err, _ := c.group.Do(key, func() (any, error) {
		if v, ok := c.fresh(key); ok {
			return v, nil
		}
		v, loadErr := load()
		if loadErr != nil {
			return nil, loadErr
		}
		c.mu.Lock()
		if c.items == nil {
			c.items = make(map[string]entry[T])
		}
		c.items[key] = entry[T]{value: v, at: time.Now()}
		c.mu.Unlock()
		return v, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return got.(T), nil
}

func (c *Cache[T]) fresh(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Since(item.at) >= c.ttl() {
		var zero T
		return zero, false
	}
	return item.value, true
}

func (c *Cache[T]) ttl() time.Duration {
	if c.TTL > 0 {
		return c.TTL
	}
	return 2 * time.Second
}
