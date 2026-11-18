package cache

import (
	"context"
	"errors"
	"time"
)

var ErrAtomicPublicationUnsupported = errors.New("cache client does not support fenced publication")

// VersionedItem atomically publishes small metadata and its body reference.
// The body is shared separately. A required owner must have been acquired
// before fetching data.
type VersionedItem struct {
	Key            string
	ReferenceKey   string
	ReferenceValue string
	BodyKey        string
	OwnerKey       string
	Token          string
	Object         any
	Expiration     time.Duration
	RequireOwner   bool
}

func (c *Cache) BeginUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if token == "" {
		return false, errors.New("empty publication owner")
	}
	if client, ok := c.GetClient().(interface {
		BeginUpdate(context.Context, string, string, time.Duration) (bool, error)
	}); ok {
		return client.BeginUpdate(ctx, c.generateFullKey(key), token, expiration)
	}
	return false, ErrAtomicPublicationUnsupported
}

func (c *Cache) ReleaseUpdate(ctx context.Context, key, token string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if client, ok := c.GetClient().(interface {
		ReleaseUpdate(context.Context, string, string) (bool, error)
	}); ok {
		return client.ReleaseUpdate(ctx, c.generateFullKey(key), token)
	}
	return false, ErrAtomicPublicationUnsupported
}

func (c *Cache) StoreVersionedItem(ctx context.Context, item VersionedItem) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if item.Object == nil || item.Token == "" || item.ReferenceValue == "" {
		return false, errors.New("empty versioned cache publication")
	}
	item.Key = c.generateFullKey(item.Key)
	item.ReferenceKey = c.generateFullKey(item.ReferenceKey)
	item.BodyKey = c.generateFullKey(item.BodyKey)
	item.OwnerKey = c.generateFullKey(item.OwnerKey)
	if client, ok := c.GetClient().(interface {
		StoreVersioned(context.Context, VersionedItem) (bool, error)
	}); ok {
		return client.StoreVersioned(ctx, item)
	}
	return false, ErrAtomicPublicationUnsupported
}

// A body is shared only while current metadata references its version.
func (c *Cache) StoreVersionedBody(ctx context.Context, referenceKey, bodyKey, referenceValue string, body any, expiration time.Duration) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if client, ok := c.GetClient().(interface {
		StoreVersionedBody(context.Context, string, string, string, any, time.Duration) (bool, error)
	}); ok {
		return client.StoreVersionedBody(ctx, c.generateFullKey(referenceKey), c.generateFullKey(bodyKey), referenceValue, body, expiration)
	}
	return false, ErrAtomicPublicationUnsupported
}
