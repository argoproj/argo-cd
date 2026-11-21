package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scache "k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	application "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// informerBackedClient simulates the manager's cache-backed client in production: reads are
// served from the informer store, while writes (and everything else) go to the wrapped API
// client.
type informerBackedClient struct {
	client.Client
	store k8scache.Store
}

func (c *informerBackedClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	item, exists, err := c.store.GetByKey(key.String())
	if err != nil {
		return err
	}
	if !exists {
		return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, key.Name)
	}
	app, ok := item.(*application.Application)
	if !ok {
		return fmt.Errorf("informer store holds unexpected type %T", item)
	}
	app.DeepCopyInto(obj.(*application.Application))
	return nil
}

func mustScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))
	return scheme
}

func mustStoreWith(t *testing.T, objs ...client.Object) k8scache.Store {
	t.Helper()
	store := k8scache.NewStore(k8scache.MetaNamespaceKeyFunc)
	for _, obj := range objs {
		require.NoError(t, store.Add(obj))
	}
	return store
}

// newCacheSyncingClient builds a cacheSyncingClient whose reads come from an informer store
// seeded with cachedObjects, whose writes go to apiClient (the fake API server), and whose
// uncached reader is also apiClient. This reproduces the production wiring
// (NewCacheSyncingClient(mgr.GetClient(), mgr.GetCache(), mgr.GetAPIReader())).
func newCacheSyncingClient(t *testing.T, apiClient client.Client, cachedObjects ...client.Object) (*cacheSyncingClient, k8scache.Store) {
	t.Helper()
	store := mustStoreWith(t, cachedObjects...)
	c := &cacheSyncingClient{
		Client:     &informerBackedClient{Client: apiClient, store: store},
		apiReader:  apiClient,
		storesByNs: map[string]k8scache.Store{},
		getNSCache: func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return &fakeMultiNamespaceCache{Store: store}, nil
		},
	}
	return c, store
}

func staleCacheGhostApp() *application.Application {
	app := &application.Application{
		Spec: application.ApplicationSpec{
			Project:    "project",
			SyncPolicy: &application.SyncPolicy{},
		},
	}
	app.Name = "app1"
	app.Namespace = "argocd"
	app.ResourceVersion = "7"
	return app
}

func desiredApp() *application.Application {
	app := &application.Application{}
	app.Name = "app1"
	app.Namespace = "argocd"
	return app
}

// TestCreateOrUpdateRecreatesAppDeletedOutOfBand is the regression test for issue #30848: the
// informer cache still holds an entry for an Application that was deleted directly on the API
// server, and the cached object matches the desired state, so the DeepEqual check alone would
// skip the write and the Application would never be recreated. The controller must confirm the
// cache hit against the API server, evict the stale entry and recreate the Application.
func TestCreateOrUpdateRecreatesAppDeletedOutOfBand(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	// The API server does NOT contain the Application; only the informer store does.
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).Build()
	c, store := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultCreated, action, "the stale cache entry must trigger recreation, not a silent skip")

	// The Application exists again on the API server.
	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Equal(t, "project", got.Spec.Project)

	// The informer store was healed: the stale entry was evicted and the recreated Application
	// synced back in by the Create.
	item, exists, err := store.GetByKey("argocd/app1")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "project", item.(*application.Application).Spec.Project)
}

// TestCreateOrUpdateSkipsUnchangedAppConfirmedByAPI guards the normal path: when the informer
// cache entry is backed by a real object on the API server, an unchanged Application is skipped
// without any write.
func TestCreateOrUpdateSkipsUnchangedAppConfirmedByAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	live := staleCacheGhostApp()
	live.ResourceVersion = "5"

	var writes int
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).WithObjects(live.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes++
				return c.Create(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				writes++
				return c.Patch(ctx, obj, patch, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				writes++
				return c.Update(ctx, obj, opts...)
			},
		}).Build()

	// Both the informer store and the API server hold the Application.
	c, _ := newCacheSyncingClient(t, apiClient, live)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)
	require.Zero(t, writes, "an unchanged Application confirmed on the API server must not be written")

	got := &application.Application{}
	require.NoError(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, got))
	require.Equal(t, "5", got.ResourceVersion, "no write should have bumped the resource version")
}

// TestCreateOrUpdateProbeErrorFallsBackToCache ensures a transient error from the uncached read
// does not fail the reconcile: the controller falls back to trusting the informer cache.
func TestCreateOrUpdateProbeErrorFallsBackToCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ghost := staleCacheGhostApp()
	probeErr := errors.New("connection refused")
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return probeErr
			},
		}).Build()
	c, _ := newCacheSyncingClient(t, apiClient, ghost)

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)

	// Nothing was created behind the API client's back.
	require.Error(t, apiClient.Get(ctx, client.ObjectKey{Name: "app1", Namespace: "argocd"}, &application.Application{}))
}

// TestCreateOrUpdateWithoutVerifierKeepsOldBehavior: a plain client (no uncached reader, e.g. in
// tests that use the fake client directly) keeps the previous behavior and skips the write.
func TestCreateOrUpdateWithoutVerifierKeepsOldBehavior(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	live := staleCacheGhostApp()
	apiClient := fake.NewClientBuilder().WithScheme(mustScheme(t)).WithObjects(live.DeepCopy()).Build()
	c, _ := newCacheSyncingClient(t, apiClient, live)
	c.apiReader = nil // no uncached reader: the cache cannot be verified

	desired := desiredApp()
	action, err := CreateOrUpdate(ctx, log.NewEntry(log.StandardLogger()), c, nil, desired, func() error {
		desired.Spec = application.ApplicationSpec{Project: "project"}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, controllerutil.OperationResultNone, action)
}
