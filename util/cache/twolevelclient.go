package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	// defaultExternalCacheCheckInterval is how often Set verifies that the external cache still holds
	// the data that the in-memory cache assumes it holds.
	defaultExternalCacheCheckInterval = 30 * time.Second
	// externalCacheMarkerExpiration bounds the lifetime of the marker key left behind by a client that stopped.
	externalCacheMarkerExpiration = 24 * time.Hour
)

// NewTwoLevelClient creates cache client that proxies requests to given external cache and tries to minimize
// number of requests to external client by storing cache entries in local in-memory cache.
func NewTwoLevelClient(client CacheClient, inMemoryExpiration time.Duration) *twoLevelClient {
	return &twoLevelClient{
		inMemoryCache:              NewInMemoryCache(inMemoryExpiration),
		externalCache:              client,
		externalCacheMarkerKey:     "twolevelclient|marker|" + uuid.NewString(),
		externalCacheCheckInterval: defaultExternalCacheCheckInterval,
	}
}

type twoLevelClient struct {
	inMemoryCache *InMemoryCache
	externalCache CacheClient
	// externalCacheMarkerKey is a key unique to this client that is kept in the external cache. If it disappears,
	// the external cache lost its data (e.g. Redis restarted without persistence) and the in-memory cache no longer
	// reflects what the external cache holds.
	externalCacheMarkerKey     string
	externalCacheCheckInterval time.Duration
	// lastExternalCacheCheck is the time of the last marker check, in Unix nanoseconds.
	lastExternalCacheCheck atomic.Int64
}

func (c *twoLevelClient) Rename(oldKey string, newKey string, expiration time.Duration) error {
	err := c.inMemoryCache.Rename(oldKey, newKey, expiration)
	if err != nil {
		log.Warnf("Failed to move key '%s' in in-memory cache: %v", oldKey, err)
	}
	return c.externalCache.Rename(oldKey, newKey, expiration)
}

// Set stores the given value in both in-memory and external cache.
// Skip storing the value in external cache if the same value already exists in memory to avoid requesting external cache,
// unless the external cache lost its data since the value was stored.
func (c *twoLevelClient) Set(item *Item) error {
	has, err := c.inMemoryCache.HasSame(item.Key, item.Object)
	if has && !c.externalCacheWasReset() {
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

// externalCacheWasReset reports whether the external cache lost the marker key, which means it no longer holds the
// values that were written through this client. In that case the in-memory cache is flushed so that subsequent
// writes reach the external cache again. A marker that is found is written again, so that it does not expire while
// the external cache keeps its data. The check runs at most once per externalCacheCheckInterval, and a failed check
// is not treated as data loss.
func (c *twoLevelClient) externalCacheWasReset() bool {
	now := time.Now().UnixNano()
	last := c.lastExternalCacheCheck.Load()
	if now-last < c.externalCacheCheckInterval.Nanoseconds() || !c.lastExternalCacheCheck.CompareAndSwap(last, now) {
		return false
	}

	var marker string
	err := c.externalCache.Get(c.externalCacheMarkerKey, &marker)
	if err == nil {
		c.saveExternalCacheMarker()
		return false
	}
	if !errors.Is(err, ErrCacheMiss) {
		log.Warnf("Failed to check marker key '%s' in external cache: %v", c.externalCacheMarkerKey, err)
		return false
	}

	// The marker is also missing on the first check after start-up, because it was never written. Flushing then
	// only costs one extra write per key.
	if last == 0 {
		log.Debugf("Marker key '%s' is not in external cache yet, flushing in-memory cache", c.externalCacheMarkerKey)
	} else {
		log.Infof("Marker key '%s' is missing from external cache, flushing in-memory cache", c.externalCacheMarkerKey)
	}
	c.inMemoryCache.Flush()
	c.saveExternalCacheMarker()
	return true
}

func (c *twoLevelClient) saveExternalCacheMarker() {
	err := c.externalCache.Set(&Item{
		Key:             c.externalCacheMarkerKey,
		Object:          c.externalCacheMarkerKey,
		CacheActionOpts: CacheActionOpts{Expiration: externalCacheMarkerExpiration},
	})
	if err != nil {
		log.Warnf("Failed to save marker key '%s' in external cache: %v", c.externalCacheMarkerKey, err)
	}
}

// Get returns cache value from in-memory cache if it present. Otherwise loads it from external cache and persists
// in memory to avoid future requests to external cache.
func (c *twoLevelClient) Get(key string, obj any) error {
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
