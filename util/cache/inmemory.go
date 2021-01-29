package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
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

// HasSame returns true if key with the same value already present in cache
func (i *InMemoryCache) HasSame(key string, obj interface{}) (bool, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(obj)
	if err != nil {
		return false, err
	}

	bufIf, found := i.memCache.Get(key)
	if !found {
		return false, nil
	}
	existingBuf, ok := bufIf.(bytes.Buffer)
	if !ok {
		panic(fmt.Errorf("InMemoryCache has unexpected entry: %v", existingBuf))
	}
	return bytes.Equal(buf.Bytes(), existingBuf.Bytes()), nil
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

// Items return a list of items in the cache; requires passing a constructor function
// so that the items can be decoded from gob format.
func (i *InMemoryCache) Items(createNewObject func() interface{}) (map[string]interface{}, error) {

	result := map[string]interface{}{}

	for key, value := range i.memCache.Items() {

		buf := value.Object.(bytes.Buffer)
		obj := createNewObject()
		err := gob.NewDecoder(&buf).Decode(obj)
		if err != nil {
			return nil, err
		}

		result[key] = obj

	}

	return result, nil
}
