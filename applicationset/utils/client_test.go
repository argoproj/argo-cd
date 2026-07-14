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

type clientOptions struct {
	objects    []client.Object
	getNSCache func(context.Context, client.Object) (ctrlcache.Cache, error)
	storesByNs map[string]k8scache.Store
}

type clientOption func(*clientOptions)

func withObjects(objs ...client.Object) clientOption {
	return func(o *clientOptions) { o.objects = objs }
}

func withGetNSCache(fn func(context.Context, client.Object) (ctrlcache.Cache, error)) clientOption {
	return func(o *clientOptions) { o.getNSCache = fn }
}

func withStoresByNs(m map[string]k8scache.Store) clientOption {
	return func(o *clientOptions) { o.storesByNs = m }
}

func newClient(t *testing.T, opts ...clientOption) (*cacheSyncingClient, k8scache.Store) {
	t.Helper()
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}

	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))

	store := k8scache.NewStore(func(obj any) (string, error) {
		return obj.(client.Object).GetName(), nil
	})
	for _, obj := range o.objects {
		require.NoError(t, store.Add(obj))
	}

	c := &cacheSyncingClient{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(o.objects...).Build(),
		storesByNs: map[string]k8scache.Store{},
		getNSCache: func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
			return &fakeMultiNamespaceCache{Store: store}, nil
		},
	}
	if o.getNSCache != nil {
		c.getNSCache = o.getNSCache
	}
	if o.storesByNs != nil {
		c.storesByNs = o.storesByNs
	}
	return c, store
}

func TestCreateSyncsCache(t *testing.T) {
	t.Parallel()
	c, store := newClient(t)

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
	c, store := newClient(t, withObjects(app))

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
	c, store := newClient(t, withObjects(app))

	require.NoError(t, c.Delete(t.Context(), app))

	require.Empty(t, store.List())
}

// TestDeleteEvictsStaleCacheOnNotFound covers the "ApplicationSet stuck in Deleting" root cause.
// The informer store still holds an Application that has already been removed from the API server
// (a missed delete watch event). Deleting it returns NotFound. The stale entry must still be
// evicted from the store, otherwise cache-backed reads keep returning the ghost object forever and
// callers such as reverse deletion never converge, only a controller restart clears it.
func TestDeleteEvictsStaleCacheOnNotFound(t *testing.T) {
	t.Parallel()

	// Seed the store with a ghost app, but leave the fake API client empty so Delete 404s.
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	c, store := newClient(t)
	require.NoError(t, store.Add(app))
	require.NotEmpty(t, store.List(), "precondition: store holds the stale entry")

	require.NoError(t, c.Delete(t.Context(), app), "a NotFound delete must be swallowed, not surfaced")

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
	c, store := newClient(t, withObjects(app))

	require.NoError(t, c.Client.Delete(t.Context(), app))
	require.Contains(t, store.List(), app)

	patchErr := c.Patch(t.Context(), app.DeepCopy(), client.MergeFrom(app))
	require.Error(t, patchErr)
	require.True(t, apierrors.IsNotFound(patchErr))
	require.Empty(t, store.List())
}

func TestUpdateNotFoundEvictsFromCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	c, store := newClient(t, withObjects(app))

	require.NoError(t, c.Client.Delete(t.Context(), app))
	require.Contains(t, store.List(), app)

	updateErr := c.Update(t.Context(), app.DeepCopy())
	require.Error(t, updateErr)
	require.True(t, apierrors.IsNotFound(updateErr))
	require.Empty(t, store.List())
}

func TestExecAndSyncCacheNotFoundSkipsNonApplication(t *testing.T) {
	t.Parallel()
	c, store := newClient(t)
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "x")
	got := c.execAndSyncCache(t.Context(), func() error { return notFound }, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "argocd"}}, false)
	require.Error(t, got)
	require.True(t, apierrors.IsNotFound(got))
	require.Empty(t, store.List())
}

func TestExecAndSyncCacheNotFoundHandlesGetStoreError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, withGetNSCache(func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
		return nil, errors.New("no cache")
	}))
	app := &application.Application{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"}}
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "test")
	require.NotPanics(t, func() {
		_ = c.execAndSyncCache(t.Context(), func() error { return notFound }, app, false)
	})
}

type errDeleteStore struct {
	k8scache.Store
}

func (e *errDeleteStore) Delete(_ any) error {
	return errors.New("delete failed")
}

func TestExecAndSyncCacheNotFoundHandlesDeleteError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, withStoresByNs(map[string]k8scache.Store{"argocd": &errDeleteStore{}}))
	app := &application.Application{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"}}
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "test")
	require.NotPanics(t, func() {
		_ = c.execAndSyncCache(t.Context(), func() error { return notFound }, app, false)
	})
}
