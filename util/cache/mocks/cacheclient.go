package mocks

import (
	"context"
	"errors"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v3/util/cache"
)

type MockCacheClient struct {
	mock.Mock
	BaseCache  cache.CacheClient
	ReadDelay  time.Duration
	WriteDelay time.Duration
}

func (c *MockCacheClient) GetContext(ctx context.Context, key string, obj any) error {
	if client, ok := c.BaseCache.(interface {
		GetContext(context.Context, string, any) error
	}); ok {
		return client.GetContext(ctx, key, obj)
	}
	return errors.New("wrapped cache does not support context-aware reads")
}

func (c *MockCacheClient) BeginUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	if client, ok := c.BaseCache.(interface {
		BeginUpdate(context.Context, string, string, time.Duration) (bool, error)
	}); ok {
		return client.BeginUpdate(ctx, key, token, expiration)
	}
	return false, cache.ErrAtomicPublicationUnsupported
}

func (c *MockCacheClient) ReleaseUpdate(ctx context.Context, key, token string) (bool, error) {
	if client, ok := c.BaseCache.(interface {
		ReleaseUpdate(context.Context, string, string) (bool, error)
	}); ok {
		return client.ReleaseUpdate(ctx, key, token)
	}
	return false, cache.ErrAtomicPublicationUnsupported
}

func (c *MockCacheClient) StoreVersioned(ctx context.Context, item cache.VersionedItem) (bool, error) {
	if client, ok := c.BaseCache.(interface {
		StoreVersioned(context.Context, cache.VersionedItem) (bool, error)
	}); ok {
		return client.StoreVersioned(ctx, item)
	}
	return false, cache.ErrAtomicPublicationUnsupported
}

func (c *MockCacheClient) StoreVersionedBody(ctx context.Context, referenceKey, bodyKey, referenceValue string, body any, expiration time.Duration) (bool, error) {
	if client, ok := c.BaseCache.(interface {
		StoreVersionedBody(context.Context, string, string, string, any, time.Duration) (bool, error)
	}); ok {
		return client.StoreVersionedBody(ctx, referenceKey, bodyKey, referenceValue, body, expiration)
	}
	return false, cache.ErrAtomicPublicationUnsupported
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

func (c *MockCacheClient) Get(key string, obj any) error {
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
