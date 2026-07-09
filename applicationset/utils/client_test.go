package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scache "k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	application "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type fakeMultiNamespaceCache struct {
	//nolint:unused
	namespaceToCache map[string]ctrlcache.Cache

	ctrlcache.Cache
	k8scache.SharedIndexInformer

	Store k8scache.Store
}

func (f *fakeMultiNamespaceCache) GetInformerForKind(_ context.Context, _ schema.GroupVersionKind, _ ...ctrlcache.InformerGetOption) (ctrlcache.Informer, error) {
	return f, nil
}

func (f *fakeMultiNamespaceCache) GetStore() k8scache.Store {
	return f.Store
}

func newClient(objs ...client.Object) (*cacheSyncingClient, k8scache.Store, error) {
	scheme := runtime.NewScheme()
	if err := application.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	store := k8scache.NewStore(func(obj any) (string, error) {
		return obj.(client.Object).GetName(), nil
	})
	for _, obj := range objs {
		if err := store.Add(obj); err != nil {
			return nil, nil, err
		}
	}
	c := &cacheSyncingClient{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
		storesByNs: map[string]k8scache.Store{},
		getNSCache: func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return &fakeMultiNamespaceCache{Store: store}, nil
		},
	}
	return c, store, nil
}

func TestCreateSyncsCache(t *testing.T) {
	t.Parallel()
	c, store, err := newClient()
	require.NoError(t, err)

	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	require.NoError(t, c.Create(t.Context(), app))

	require.Contains(t, store.List(), app)
}

func TestUpdateSyncsCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "argocd",
			Labels:    map[string]string{"foo": "bar"},
		},
	}
	c, store, err := newClient(app)
	require.NoError(t, err)

	updatedApp := app.DeepCopy()
	updatedApp.Labels["foo"] = "bar-UPDATED"
	require.NoError(t, c.Update(t.Context(), updatedApp))

	updated, _, err := store.GetByKey("test")
	require.NoError(t, err)
	require.Equal(t, "bar-UPDATED", updated.(*application.Application).Labels["foo"])
}

func TestDeleteSyncsCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "argocd",
			Labels:    map[string]string{"foo": "bar"},
		},
	}
	c, store, err := newClient(app)
	require.NoError(t, err)

	require.NoError(t, c.Delete(t.Context(), app))

	require.Empty(t, store.List())
}

// TestDeleteEvictsStaleCacheOnNotFound covers the "ApplicationSet stuck in Deleting" root cause.
// The informer store still holds an Application that has already been removed from the API server
// (a missed delete watch event). Deleting it returns NotFound. The stale entry must still be
// evicted from the store, otherwise cache-backed reads keep returning the ghost object forever and
// callers such as reverse deletion never converge — only a controller restart clears it.
func TestDeleteEvictsStaleCacheOnNotFound(t *testing.T) {
	t.Parallel()

	// Seed the store with a ghost app, but leave the fake API client empty so Delete 404s.
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	c, store, err := newClient()
	require.NoError(t, err)
	require.NoError(t, store.Add(app))
	require.NotEmpty(t, store.List(), "precondition: store holds the stale entry")

	err = c.Delete(t.Context(), app)
	require.NoError(t, err, "a NotFound delete must be swallowed, not surfaced")

	require.Empty(t, store.List(), "stale cache entry should be evicted even though the API returned NotFound")
}

func TestNewClientDoesNotCrashWithMultiNamespaceCache(_ *testing.T) {
	_ = NewCacheSyncingClient(nil, &fakeMultiNamespaceCache{})
}

func TestPatchNotFoundEvictsFromCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	c, store, err := newClient(app)
	require.NoError(t, err)

	require.NoError(t, c.Client.Delete(t.Context(), app))
	require.Contains(t, store.List(), app)

	patchErr := c.Patch(t.Context(), app.DeepCopy(), client.MergeFrom(app))
	require.Error(t, patchErr)
	require.True(t, apierrors.IsNotFound(patchErr))
	require.Empty(t, store.List())
}

func TestEvictFromCacheSkipsNonApplication(t *testing.T) {
	t.Parallel()
	c, store, err := newClient()
	require.NoError(t, err)
	c.evictFromCache(t.Context(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "argocd"}})
	require.Empty(t, store.List())
}

func TestEvictFromCacheHandlesGetStoreError(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))
	c := &cacheSyncingClient{
		Client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		storesByNs: map[string]k8scache.Store{},
		getNSCache: func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return nil, errors.New("no cache")
		},
	}
	app := &application.Application{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"}}
	require.NotPanics(t, func() { c.evictFromCache(t.Context(), app) })
}

type errDeleteStore struct {
	k8scache.Store
}

func (e *errDeleteStore) Delete(_ any) error {
	return errors.New("delete failed")
}

func TestEvictFromCacheHandlesDeleteError(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))
	c := &cacheSyncingClient{
		Client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		storesByNs: map[string]k8scache.Store{"argocd": &errDeleteStore{}},
	}
	app := &application.Application{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"}}
	require.NotPanics(t, func() { c.evictFromCache(t.Context(), app) })
}
