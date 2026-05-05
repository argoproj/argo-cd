package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
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
	c, store, err := newClient()
	require.NoError(t, err)

	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "argocd"},
	}
	require.NoError(t, c.Create(context.Background(), app))

	require.Contains(t, store.List(), app)
}

func TestUpdateSyncsCache(t *testing.T) {
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
	require.NoError(t, c.Update(context.Background(), updatedApp))

	updated, _, err := store.GetByKey("test")
	require.NoError(t, err)
	require.Equal(t, "bar-UPDATED", updated.(*application.Application).Labels["foo"])
}

func TestDeleteSyncsCache(t *testing.T) {
	app := &application.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "argocd",
			Labels:    map[string]string{"foo": "bar"},
		},
	}
	c, store, err := newClient(app)
	require.NoError(t, err)

	require.NoError(t, c.Delete(context.Background(), app))

	require.Empty(t, store.List())
}

func TestNewClientDoesNotCrashWithMultiNamespaceCache(_ *testing.T) {
	_ = NewCacheSyncingClient(nil, &fakeMultiNamespaceCache{})
}
