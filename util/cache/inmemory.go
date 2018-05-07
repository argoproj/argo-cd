package cache

import (
	"bytes"
	"encoding/gob"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

func NewInMemoryCache(expiration time.Duration) Cache {
	return &inMemoryCache{
		memCache: gocache.New(expiration, 1*time.Minute),
	}
}

type inMemoryCache struct {
	memCache *gocache.Cache
}

func (i *inMemoryCache) Set(item *Item) error {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(item.Object)
	if err != nil {
		return err
	}
	i.memCache.Set(item.Key, buf, item.Expiration)
	return nil
}

func (i *inMemoryCache) Get(key string, obj interface{}) error {
	bufIf, found := i.memCache.Get(key)
	if !found {
		return ErrCacheMiss
	}
	buf := bufIf.(bytes.Buffer)
	return gob.NewDecoder(&buf).Decode(obj)
}
