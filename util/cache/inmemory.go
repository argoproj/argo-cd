package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

func NewInMemoryCache(expiration time.Duration) *InMemoryCache {
	return &InMemoryCache{
		memCache: gocache.New(expiration, 1*time.Minute),
	}
}

type InMemoryCache struct {
	memCache *gocache.Cache
}

func (i *InMemoryCache) Set(item *Item) error {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(item.Object)
	if err != nil {
		return err
	}
	i.memCache.Set(item.Key, buf, item.Expiration)
	return nil
}

func (i *InMemoryCache) Get(key string, obj interface{}) error {
	bufIf, found := i.memCache.Get(key)
	if !found {
		return ErrCacheMiss
	}
	buf := bufIf.(bytes.Buffer)
	return gob.NewDecoder(&buf).Decode(obj)
}

func (i *InMemoryCache) Delete(key string) error {
	i.memCache.Delete(key)
	return nil
}

func (i *InMemoryCache) Flush() {
	i.memCache.Flush()
}

func (i *InMemoryCache) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return nil
}

func (i *InMemoryCache) NotifyUpdated(key string) error {
	return nil
}
