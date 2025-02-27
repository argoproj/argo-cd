package cache

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// NewTwoLevelClient creates cache client that proxies requests to given external cache and tries to minimize
// number of requests to external client by storing cache entries in local in-memory cache.
func NewTwoLevelClient(client CacheClient, inMemoryExpiration time.Duration) *twoLevelClient {
	return &twoLevelClient{inMemoryCache: NewInMemoryCache(inMemoryExpiration), externalCache: client}
}

type twoLevelClient struct {
	inMemoryCache *InMemoryCache
	externalCache CacheClient
}

func (c *twoLevelClient) Rename(oldKey string, newKey string, expiration time.Duration) error {
	err := c.inMemoryCache.Rename(oldKey, newKey, expiration)
	if err != nil {
		log.Warnf("Failed to move key '%s' in in-memory cache: %v", oldKey, err)
	}
	return c.externalCache.Rename(oldKey, newKey, expiration)
}

// Set stores the given value in both in-memory and external cache.
// Skip storing the value in external cache if the same value already exists in memory to avoid requesting external cache.
func (c *twoLevelClient) Set(item *Item) error {
	has, err := c.inMemoryCache.HasSame(item.Key, item.Object)
	if has {
		return nil
	}
	if err != nil {
		log.Warnf("Failed to check key '%s' in in-memory cache: %v", item.Key, err)
	}
	err = c.inMemoryCache.Set(item)
	if err != nil {
		log.Warnf("Failed to save key '%s' in in-memory cache: %v", item.Key, err)
	}
	return c.externalCache.Set(item)
}

// Get returns cache value from in-memory cache if it present. Otherwise loads it from external cache and persists
// in memory to avoid future requests to external cache.
func (c *twoLevelClient) Get(key string, obj interface{}) error {
	err := c.inMemoryCache.Get(key, obj)
	if err == nil {
		return nil
	}

	err = c.externalCache.Get(key, obj)
	if err == nil {
		_ = c.inMemoryCache.Set(&Item{Key: key, Object: obj})
	}
	return err
}

// Delete deletes cache for given key in both in-memory and external cache.
func (c *twoLevelClient) Delete(key string) error {
	err := c.inMemoryCache.Delete(key)
	if err != nil {
		return err
	}
	return c.externalCache.Delete(key)
}

func (c *twoLevelClient) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.externalCache.OnUpdated(ctx, key, callback)
}

func (c *twoLevelClient) NotifyUpdated(key string) error {
	return c.externalCache.NotifyUpdated(key)
}
