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

// compile-time validation of adherance of the CacheClient contract
var _ CacheClient = &twoLevelClient{}

type twoLevelClient struct {
	inMemoryCache *InMemoryCache
	externalCache CacheClient
}

// Set stores the given value in both in-memory and external cache.
// Skip storing the value in external cache if the same value already exists in memory to avoid requesting external cache.
func (c *twoLevelClient) Set(item *Item) error {
	switch item.CacheType {
	case CacheTypeInMemory:
		return c.inMemoryCache.Set(item)
	case CacheTypeExternal:
		return c.externalCache.Set(item)
	default:
		return c.SetTwoLevel(item)
	}
}

func (c *twoLevelClient) SetTwoLevel(item *Item) error {
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

func (c *twoLevelClient) Get(item *Item) error {
	switch item.CacheType {
	case CacheTypeInMemory:
		return c.inMemoryCache.Get(item)
	case CacheTypeExternal:
		return c.externalCache.Get(item)
	default:
		return c.GetTwoLevel(item)
	}
}

// Get returns cache value from in-memory cache if it present. Otherwise loads it from external cache and persists
// in memory to avoid future requests to external cache.
func (c *twoLevelClient) GetTwoLevel(item *Item) error {
	err := c.inMemoryCache.Get(item)
	if err == nil {
		return nil
	}

	err = c.externalCache.Get(item)
	if err == nil {
		_ = c.inMemoryCache.Set(item)
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
