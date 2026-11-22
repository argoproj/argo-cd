package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
		Name: "test", Namespace: "argocd",
	}
	require.NoError(t, c.Create(t.Context(), app))

	require.Contains(t, store.List(), app)
}

func TestUpdateSyncsCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:      "test",
		Namespace: "argocd",
		Labels:    map[string]string{"foo": "bar"},
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
		Name:      "test",
		Namespace: "argocd",
		Labels:    map[string]string{"foo": "bar"},
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
		Name: "test", Namespace: "argocd",
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
		Name: "test", Namespace: "argocd",
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
		Name: "test", Namespace: "argocd",
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
	got := c.execAndSyncCache(t.Context(), func() error { return notFound }, &corev1.ConfigMap{Name: "x", Namespace: "argocd"}, syncRefresh)
	require.Error(t, got)
	require.True(t, apierrors.IsNotFound(got))
	require.Empty(t, store.List())
}

func TestExecAndSyncCacheNotFoundHandlesGetStoreError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, withGetNSCache(func(_ context.Context, _ client.Object) (ctrlcache.Cache, error) {
		return nil, errors.New("no cache")
	}))
	app := &application.Application{Name: "test", Namespace: "argocd"}
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "test")
	require.NotPanics(t, func() {
		_ = c.execAndSyncCache(t.Context(), func() error { return notFound }, app, syncRefresh)
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
	app := &application.Application{Name: "test", Namespace: "argocd"}
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "test")
	require.NotPanics(t, func() {
		_ = c.execAndSyncCache(t.Context(), func() error { return notFound }, app, syncRefresh)
	})
}

// TestDeleteKeepsCacheEntryWhileFinalizersPending covers the premature-release defect. Deleting an
// Application that carries finalizers does not remove it: the API server only stamps
// deletionTimestamp and the object lives on. Evicting the store entry anyway makes the next
// cache-backed read report the Application as gone, so reverse deletion sees an empty list, logs
// "completed reverse deletion" and drops the ApplicationSet's finalizer while children are still
// terminating — ordered teardown silently stops being enforced.
func TestDeleteKeepsCacheEntryWhileFinalizersPending(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:       "test",
		Namespace:  "argocd",
		Finalizers: []string{application.ResourcesFinalizerName},
	}
	c, store := newClient(t, withObjects(app))

	require.NoError(t, c.Delete(t.Context(), app))

	require.Len(t, store.List(), 1, "an Application held by finalizers is still terminating and must stay in the cache")
}

// TestUpdateDoesNotResurrectDeletedCacheEntry covers the resurrection race from issue #30848: a
// successful Update whose response is synced into the informer store after the informer already
// processed the object's DELETED event must not re-add the entry. Otherwise cache-backed reads
// keep returning a ghost object that no longer exists on the API server, and no further watch
// event will ever remove it.
func TestUpdateDoesNotResurrectDeletedCacheEntry(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:      "test",
		Namespace: "argocd",
		Labels:    map[string]string{"foo": "bar"},
	}
	c, store := newClient(t, withObjects(app))

	// The informer processed the DELETED event (entry gone from the store), but the object still
	// exists on the API server, so the Update below succeeds.
	require.NoError(t, store.Delete(app))

	updated := app.DeepCopy()
	updated.Labels["foo"] = "bar-UPDATED"
	require.NoError(t, c.Update(t.Context(), updated))

	require.Empty(t, store.List(), "a successful Update must not re-add an entry the informer already deleted")
}

// TestPatchDoesNotResurrectDeletedCacheEntry is the Patch variant of the race above.
func TestPatchDoesNotResurrectDeletedCacheEntry(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:      "test",
		Namespace: "argocd",
		Labels:    map[string]string{"foo": "bar"},
	}
	c, store := newClient(t, withObjects(app))

	require.NoError(t, store.Delete(app))

	patched := app.DeepCopy()
	patched.Labels["foo"] = "bar-PATCHED"
	require.NoError(t, c.Patch(t.Context(), patched, client.MergeFrom(app)))

	require.Empty(t, store.List(), "a successful Patch must not re-add an entry the informer already deleted")
}

// TestPatchStillRefreshesExistingCacheEntry guards the normal path: when the entry is still in the
// store, a successful Patch refreshes it.
func TestPatchStillRefreshesExistingCacheEntry(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:      "test",
		Namespace: "argocd",
		Labels:    map[string]string{"foo": "bar"},
	}
	c, store := newClient(t, withObjects(app))

	patched := app.DeepCopy()
	patched.Labels["foo"] = "bar-PATCHED"
	require.NoError(t, c.Patch(t.Context(), patched, client.MergeFrom(app)))

	got, exists, err := store.GetByKey("test")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "bar-PATCHED", got.(*application.Application).Labels["foo"])
}

func TestEvictFromCache(t *testing.T) {
	t.Parallel()
	app := &application.Application{
		Name:      "test",
		Namespace: "argocd",
	}
	c, store := newClient(t, withObjects(app))
	require.NotEmpty(t, store.List())

	c.EvictFromCache(t.Context(), app)

	require.Empty(t, store.List(), "EvictFromCache should remove the entry from the informer store")
}

func TestUncachedReader(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t)
	require.Nil(t, c.UncachedReader())

	apiReader := fake.NewClientBuilder().Build()
	cWithReader := NewCacheSyncingClient(c.Client, &fakeMultiNamespaceCache{}, apiReader)
	verifier, ok := cWithReader.(interface{ UncachedReader() client.Reader })
	require.True(t, ok)
	require.Equal(t, client.Reader(apiReader), verifier.UncachedReader())
}

// TestEvictFromCacheKeepsReplacementEntry: between the uncached probe that confirmed the object
// is gone and the eviction, the informer may have stored a replacement object for the same key.
// Eviction must remove only the stale entry (matched by UID), not the replacement.
func TestEvictFromCacheKeepsReplacementEntry(t *testing.T) {
	t.Parallel()
	stale := &application.Application{Name: "test", Namespace: "argocd", ResourceVersion: "7"}
	stale.UID = "uid-stale"
	c, store := newClient(t, withObjects(stale))

	replacement := &application.Application{Name: "test", Namespace: "argocd", ResourceVersion: "8"}
	replacement.UID = "uid-new"
	require.NoError(t, store.Update(replacement))

	c.EvictFromCache(t.Context(), stale)

	got, exists, err := store.Get(stale)
	require.NoError(t, err)
	require.True(t, exists, "the replacement entry must not be evicted")
	require.Equal(t, types.UID("uid-new"), got.(*application.Application).UID)
}

// TestCreateAlreadyExistsSyncsCache: when a Create loses an AlreadyExists race, the existing
// object must be synced into the informer store so cache-backed reads stop reporting it missing.
func TestCreateAlreadyExistsSyncsCache(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, application.AddToScheme(scheme))

	existing := &application.Application{Name: "test", Namespace: "argocd", Labels: map[string]string{"a": "b"}}
	existing.UID = "uid-live"
	apiClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	c, store := newClient(t)
	c.Client = apiClient
	c.apiReader = apiClient // the informer store starts empty: the informer has not seen the object yet

	err := c.Create(t.Context(), &application.Application{Name: "test", Namespace: "argocd"})
	require.True(t, apierrors.IsAlreadyExists(err), "expected AlreadyExists, got %v", err)

	got, exists, err := store.GetByKey("test")
	require.NoError(t, err)
	require.True(t, exists, "the existing object must be synced into the informer store after AlreadyExists")
	require.Equal(t, types.UID("uid-live"), got.(*application.Application).UID)
	require.Equal(t, "b", got.(*application.Application).Labels["a"])
}
