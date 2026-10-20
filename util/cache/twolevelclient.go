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
	// externalCacheMarkerWrittenAt is the time of the last successful marker write, in Unix nanoseconds.
	externalCacheMarkerWrittenAt atomic.Int64
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
// unless the external cache lost its data since the value was stored. If the external write fails, the value is
// removed from memory so that the next Set with the same value writes it again.
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
	err = c.externalCache.Set(item)
	if err != nil {
		// Drop the in-memory copy so that a retry with the same value is not skipped.
		_ = c.inMemoryCache.Delete(item.Key)
	}
	return err
}

// externalCacheWasReset reports whether the external cache lost the marker key, which means it no longer holds the
// values that were written through this client. In that case the in-memory cache is flushed so that subsequent
// writes reach the external cache again. The check runs at most once per externalCacheCheckInterval, and a failed
// check is not treated as data loss.
func (c *twoLevelClient) externalCacheWasReset() bool {
	now := time.Now().UnixNano()
	last := c.lastExternalCacheCheck.Load()
	if now-last < c.externalCacheCheckInterval.Nanoseconds() || !c.lastExternalCacheCheck.CompareAndSwap(last, now) {
		return false
	}

	var marker string
	err := c.externalCache.Get(c.externalCacheMarkerKey, &marker)
	if err == nil {
		return false
	}
	if !errors.Is(err, ErrCacheMiss) {
		log.Warnf("Failed to check marker key '%s' in external cache: %v", c.externalCacheMarkerKey, err)
		return false
	}

	// A marker written less than externalCacheMarkerExpiration ago can only be missing because the external cache lost
	// it. Otherwise it was never written (first check after start-up) or it expired, and the flush is a precaution
	// that costs one extra write per key. Data lost after the marker expired but before this check is logged as an
	// expiry; the flush is the same either way.
	writtenAt := c.externalCacheMarkerWrittenAt.Load()
	if writtenAt != 0 && now-writtenAt < externalCacheMarkerExpiration.Nanoseconds() {
		log.Infof("Marker key '%s' is missing from external cache, flushing in-memory cache", c.externalCacheMarkerKey)
	} else {
		log.Debugf("Marker key '%s' is not in external cache yet or expired, flushing in-memory cache", c.externalCacheMarkerKey)
	}
	c.inMemoryCache.Flush()

	// Take the time before the write, so that the marker always expires in the external cache after
	// externalCacheMarkerExpiration has passed by this clock.
	writeTime := time.Now().UnixNano()
	err = c.externalCache.Set(&Item{
		Key:             c.externalCacheMarkerKey,
		Object:          c.externalCacheMarkerKey,
		CacheActionOpts: CacheActionOpts{Expiration: externalCacheMarkerExpiration},
	})
	if err != nil {
		log.Warnf("Failed to save marker key '%s' in external cache: %v", c.externalCacheMarkerKey, err)
		return true
	}
	c.externalCacheMarkerWrittenAt.Store(writeTime)
	return true
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
