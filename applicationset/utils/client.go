package utils

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8scache "k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	application "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// NewCacheSyncingClient returns a client that wraps the given client and syncs the cache after each Create, Update, Patch, or Delete operation on Application objects.
func NewCacheSyncingClient(c client.Client, cache ctrlcache.Cache) client.Client {
	res := &cacheSyncingClient{Client: c, storesByNs: make(map[string]k8scache.Store)}
	// The k8s controller runtime's cache does not expose a way to get the underlying store, so we have to use reflection to access it.
	// This is necessary to keep the cache in sync with the client operations.
	field := reflect.ValueOf(cache).Elem().FieldByName("namespaceToCache")
	if field.Kind() == reflect.Map {
		namespaceToCache := *(*map[string]ctrlcache.Cache)(unsafe.Pointer(field.UnsafeAddr()))
		res.getNSCache = func(_ context.Context, obj client.Object) (ctrlcache.Cache, error) {
			res, ok := namespaceToCache[obj.GetNamespace()]
			if !ok {
				return nil, fmt.Errorf("cache for namespace %s not found", obj.GetNamespace())
			}
			return res, nil
		}
	} else {
		res.getNSCache = func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return cache, nil
		}
	}
	return res
}

type cacheSyncingClient struct {
	client.Client
	getNSCache func(ctx context.Context, obj client.Object) (ctrlcache.Cache, error)

	storesByNs     map[string]k8scache.Store
	storesByNsLock sync.RWMutex
}

func (c *cacheSyncingClient) getStore(ctx context.Context, obj client.Object) (k8scache.Store, error) {
	c.storesByNsLock.RLock()
	store, ok := c.storesByNs[obj.GetNamespace()]
	c.storesByNsLock.RUnlock()
	if ok {
		return store, nil
	}

	store, err := c.retrieveStore(ctx, obj)
	if err != nil {
		return nil, err
	}

	c.storesByNsLock.Lock()
	c.storesByNs[obj.GetNamespace()] = store
	c.storesByNsLock.Unlock()

	return store, nil
}

func (c *cacheSyncingClient) retrieveStore(ctx context.Context, obj client.Object) (k8scache.Store, error) {
	nsCache, err := c.getNSCache(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace cache: %w", err)
	}

	informer, err := nsCache.GetInformerForKind(ctx, application.ApplicationSchemaGroupVersionKind)
	if err != nil {
		return nil, fmt.Errorf("failed to get informer: %w", err)
	}
	indexInformer, ok := informer.(k8scache.SharedIndexInformer)
	if !ok {
		return nil, errors.New("informer is not a SharedIndexInformer")
	}

	return indexInformer.GetStore(), nil
}

func (c *cacheSyncingClient) execAndSyncCache(ctx context.Context, op func() error, obj client.Object, deleteObj bool) error {
	// execute the operation first and only sync cache if it succeeds
	var opErr error
	if err := op(); err != nil {
		// A NotFound means the object is already gone from the API server. Fall through to
		// evict any stale entry from the informer cache below, otherwise a lingering
		// (already-deleted) object keeps being read back and callers can never converge.
		// Delete callers already expected deletion, so we swallow the error (idempotent).
		// Update/Patch callers still need to know the object is gone, so we return NotFound.
		if !apierrors.IsNotFound(err) {
			return err
		}
		if !deleteObj {
			opErr = err
		}
		deleteObj = true
	}
	// sync cache for applications only
	if _, ok := obj.(*application.Application); !ok {
		return opErr
	}

	logger := log.WithField("namespace", obj.GetNamespace()).WithField("name", obj.GetName())
	store, err := c.getStore(ctx, obj)
	if err != nil {
		logger.Errorf("failed to get cache store: %v", err)
	} else {
		if deleteObj {
			err = store.Delete(obj)
		} else {
			err = store.Update(obj)
		}
	}
	if err != nil {
		logger.Errorf("failed to sync cache for object: %v", err)
	}
	return opErr
}

func (c *cacheSyncingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.execAndSyncCache(ctx, func() error {
		return c.Client.Create(ctx, obj, opts...)
	}, obj, false)
}

func (c *cacheSyncingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.execAndSyncCache(ctx, func() error {
		return c.Client.Update(ctx, obj, opts...)
	}, obj, false)
}

func (c *cacheSyncingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.execAndSyncCache(ctx, func() error {
		return c.Client.Patch(ctx, obj, patch, opts...)
	}, obj, false)
}

func (c *cacheSyncingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.execAndSyncCache(ctx, func() error {
		return c.Client.Delete(ctx, obj, opts...)
	}, obj, true)
}
