package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/util/cache"
)

type MockCacheClient struct {
	mock.Mock
	BaseCache  cache.CacheClient
	ReadDelay  time.Duration
	WriteDelay time.Duration
}

func (c *MockCacheClient) Rename(oldKey string, newKey string, expiration time.Duration) error {
	args := c.Called(oldKey, newKey, expiration)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	return c.BaseCache.Rename(oldKey, newKey, expiration)
}

func (c *MockCacheClient) Set(item *cache.Item) error {
	args := c.Called(item)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	if c.WriteDelay > 0 {
		time.Sleep(c.WriteDelay)
	}
	return c.BaseCache.Set(item)
}

func (c *MockCacheClient) Get(key string, obj interface{}) error {
	args := c.Called(key, obj)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	if c.ReadDelay > 0 {
		time.Sleep(c.ReadDelay)
	}
	return c.BaseCache.Get(key, obj)
}

func (c *MockCacheClient) Delete(key string) error {
	args := c.Called(key)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	if c.WriteDelay > 0 {
		time.Sleep(c.WriteDelay)
	}
	return c.BaseCache.Delete(key)
}

func (c *MockCacheClient) OnUpdated(ctx context.Context, key string, callback func() error) error {
	args := c.Called(ctx, key, callback)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	return c.BaseCache.OnUpdated(ctx, key, callback)
}

func (c *MockCacheClient) NotifyUpdated(key string) error {
	args := c.Called(key)
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(error)
	}
	return c.BaseCache.NotifyUpdated(key)
}
