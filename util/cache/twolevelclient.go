package cache

import (
	"context"
	"time"
)

func NewTwoLevelClient(client CacheClient, inMemoryExpiration time.Duration) *twoLevelClient {
	return &twoLevelClient{inMemoryCache: NewInMemoryCache(inMemoryExpiration), client: client}
}

type twoLevelClient struct {
	inMemoryCache *InMemoryCache
	client        CacheClient
}

func (c *twoLevelClient) Set(item *Item) error {
	if c.inMemoryCache.HasSame(item.Key, item.Object) {
		return nil
	}
	_ = c.inMemoryCache.Set(item)
	return c.client.Set(item)
}

func (c *twoLevelClient) Get(key string, obj interface{}) error {
	err := c.inMemoryCache.Get(key, obj)
	if err == nil {
		return nil
	}

	err = c.client.Get(key, obj)
	if err == nil {
		_ = c.inMemoryCache.Set(&Item{Key: key, Object: obj})
	}
	return err
}

func (c *twoLevelClient) Delete(key string) error {
	err := c.inMemoryCache.Delete(key)
	if err != nil {
		return err
	}
	return c.client.Delete(key)
}

func (c *twoLevelClient) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.client.OnUpdated(ctx, key, callback)
}

func (c *twoLevelClient) NotifyUpdated(key string) error {
	return c.client.NotifyUpdated(key)
}
