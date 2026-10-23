package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
)

// drainKeys returns all keys currently queued, marking each as done.
func drainKeys(q workqueue.TypedRateLimitingInterface[string]) []string {
	var keys []string
	for q.Len() > 0 {
		key, _ := q.Get()
		keys = append(keys, key)
		q.Done(key)
	}
	return keys
}

// assertEnqueued waits until exactly one item is queued and returns it. Items added
// via AddRateLimited go through the delaying queue (1ms base delay), so polling is
// required.
func assertEnqueued(t *testing.T, q workqueue.TypedRateLimitingInterface[string]) string {
	t.Helper()
	require.Eventually(t, func() bool { return q.Len() >= 1 }, 2*time.Second, 5*time.Millisecond, "expected an item to be enqueued")
	keys := drainKeys(q)
	require.Len(t, keys, 1)
	return keys[0]
}

// assertNotEnqueued asserts that nothing is queued within a short window.
func assertNotEnqueued(t *testing.T, q workqueue.TypedRateLimitingInterface[string]) {
	t.Helper()
	assert.Never(t, func() bool { return q.Len() > 0 }, 200*time.Millisecond, 20*time.Millisecond, "expected nothing to be enqueued")
}

func newProject(name string) *v1alpha1.AppProject {
	return &v1alpha1.AppProject{Name: name, Namespace: test.FakeArgoCDNamespace}
}

func TestAppProjectEventHandlerFuncs(t *testing.T) {
	t.Run("add requeues the project and invalidates its cache entry", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		ctrl.projByNameCache.Store("my-proj", struct{}{})
		h := ctrl.appProjectEventHandlerFuncs()

		h.OnAdd(newProject("my-proj"), false)

		assert.Contains(t, assertEnqueued(t, ctrl.projectRefreshQueue), "my-proj")
		_, ok := ctrl.projByNameCache.Load("my-proj")
		assert.False(t, ok, "project cache entry should have been invalidated")
	})

	t.Run("update requeues the project and invalidates its cache entry", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		ctrl.projByNameCache.Store("my-proj", struct{}{})
		h := ctrl.appProjectEventHandlerFuncs()

		h.OnUpdate(newProject("my-proj"), newProject("my-proj"))

		assert.Contains(t, assertEnqueued(t, ctrl.projectRefreshQueue), "my-proj")
		_, ok := ctrl.projByNameCache.Load("my-proj")
		assert.False(t, ok)
	})

	t.Run("delete requeues the project and invalidates its cache entry", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		ctrl.projByNameCache.Store("my-proj", struct{}{})
		h := ctrl.appProjectEventHandlerFuncs()

		h.OnDelete(newProject("my-proj"))

		assert.Contains(t, assertEnqueued(t, ctrl.projectRefreshQueue), "my-proj")
		_, ok := ctrl.projByNameCache.Load("my-proj")
		assert.False(t, ok)
	})

	t.Run("delete unwraps a tombstone wrapping a project", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		ctrl.projByNameCache.Store("my-proj", struct{}{})
		h := ctrl.appProjectEventHandlerFuncs()

		h.OnDelete(cache.DeletedFinalStateUnknown{Key: test.FakeArgoCDNamespace + "/my-proj", Obj: newProject("my-proj")})

		assert.Contains(t, assertEnqueued(t, ctrl.projectRefreshQueue), "my-proj")
		_, ok := ctrl.projByNameCache.Load("my-proj")
		assert.False(t, ok)
	})

	t.Run("delete of a tombstone wrapping nil is ignored without panicking", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		ctrl.projByNameCache.Store("my-proj", struct{}{})
		h := ctrl.appProjectEventHandlerFuncs()

		assert.NotPanics(t, func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: test.FakeArgoCDNamespace + "/gone", Obj: nil})
		})
		assertNotEnqueued(t, ctrl.projectRefreshQueue)
		_, ok := ctrl.projByNameCache.Load("my-proj")
		assert.True(t, ok, "unrelated cache entry should be untouched")
	})

	t.Run("unexpected object types are ignored without panicking", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.appProjectEventHandlerFuncs()

		assert.NotPanics(t, func() {
			h.OnAdd("not-a-project", false)
			h.OnUpdate("not-a-project", "not-a-project")
			h.OnDelete("not-a-project")
		})
		assertNotEnqueued(t, ctrl.projectRefreshQueue)
	})
}

func TestApplicationEventHandlerFuncs(t *testing.T) {
	t.Run("add of a processable application requeues a refresh", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		h.OnAdd(newFakeApp(), false)

		assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
	})

	t.Run("update of a processable application requeues a refresh", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		old := newFakeApp()
		updated := newFakeApp()
		updated.ResourceVersion = "2"
		h.OnUpdate(old, updated)

		assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
		assertNotEnqueued(t, ctrl.appOperationQueue)
	})

	t.Run("update of an application with an ongoing operation requeues an operation", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		old := newFakeApp()
		updated := newFakeApp()
		updated.ResourceVersion = "2"
		updated.Operation = &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{}}
		h.OnUpdate(old, updated)

		assert.Contains(t, assertEnqueued(t, ctrl.appOperationQueue), "my-app")
	})

	t.Run("update of a terminating application requeues an operation", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		now := metav1.Now()
		old := newFakeApp()
		updated := newFakeApp()
		updated.ResourceVersion = "2"
		updated.DeletionTimestamp = &now
		h.OnUpdate(old, updated)

		assert.Contains(t, assertEnqueued(t, ctrl.appOperationQueue), "my-app")
	})

	t.Run("delete of a processable application requeues a refresh", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		h.OnDelete(newFakeApp())

		assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
	})

	t.Run("delete unwraps a tombstone wrapping an application", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		app := newFakeApp()
		h.OnDelete(cache.DeletedFinalStateUnknown{Key: app.Namespace + "/" + app.Name, Obj: app})

		assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
	})

	t.Run("delete of a tombstone wrapping nil is ignored without panicking", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		assert.NotPanics(t, func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "gone", Obj: nil})
		})
		assertNotEnqueued(t, ctrl.appRefreshQueue)
	})

	t.Run("an application in a disallowed namespace is ignored", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		app := newFakeApp()
		app.Namespace = "some-other-namespace"
		h.OnAdd(app, false)
		h.OnUpdate(app, app)
		h.OnDelete(app)

		assertNotEnqueued(t, ctrl.appRefreshQueue)
	})

	t.Run("unexpected object types are ignored without panicking", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		assert.NotPanics(t, func() {
			h.OnAdd("not-an-app", false)
			h.OnUpdate("not-an-app", "not-an-app")
			h.OnDelete("not-an-app")
		})
		assertNotEnqueued(t, ctrl.appRefreshQueue)
	})
}

// withSharedTreeCache gives ctrl a two-level cache client, like the real controller, backed by an
// external cache that a second controller also writes to. It returns that second controller's cache
// and a reader of the external cache.
func withSharedTreeCache(ctrl *ApplicationController) (other, reader *appstatecache.Cache) {
	external := cacheutil.NewInMemoryCache(time.Hour) // stands in for Redis
	ctrl.cache.Cache.SetClient(cacheutil.NewTwoLevelClient(external, time.Hour))
	other = appstatecache.NewCache(cacheutil.NewCache(cacheutil.NewTwoLevelClient(external, time.Hour)), time.Hour)
	reader = appstatecache.NewCache(cacheutil.NewCache(external), time.Hour)
	return other, reader
}

func TestApplicationEventHandlerFuncs_InvalidatesLocalCacheOnArrival(t *testing.T) {
	healthy := &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{Kind: "Pod", Name: "pod-1"}}}
	suspended := &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{Kind: "Deployment", Name: "deploy"}}}

	// The app was on this controller, then another controller overwrote its tree, then it comes back
	// via the given event. The next write of the unchanged tree must reach the external cache.
	run := func(t *testing.T, arrive func(h cache.ResourceEventHandlerFuncs, app *v1alpha1.Application)) string {
		t.Helper()
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		other, reader := withSharedTreeCache(ctrl)
		app := newFakeApp()
		name := app.InstanceName(ctrl.namespace)

		require.NoError(t, ctrl.cache.SetAppResourcesTree(name, healthy))
		require.NoError(t, other.SetAppResourcesTree(name, suspended))
		arrive(ctrl.applicationEventHandlerFuncs(), app)
		require.NoError(t, ctrl.cache.SetAppResourcesTree(name, healthy))

		got := &v1alpha1.ApplicationTree{}
		require.NoError(t, reader.GetAppResourcesTree(name, got))
		require.Len(t, got.Nodes, 1)
		return got.Nodes[0].Name
	}

	t.Run("add of a processable application", func(t *testing.T) {
		name := run(t, func(h cache.ResourceEventHandlerFuncs, app *v1alpha1.Application) {
			h.OnAdd(app, false)
		})
		assert.Equal(t, "pod-1", name)
	})

	t.Run("update that makes the application processable", func(t *testing.T) {
		name := run(t, func(h cache.ResourceEventHandlerFuncs, app *v1alpha1.Application) {
			old := app.DeepCopy()
			// Stands in for "previously handled by another controller", e.g. a destination on another shard.
			old.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}
			app.ResourceVersion = "2"
			h.OnUpdate(old, app)
		})
		assert.Equal(t, "pod-1", name)
	})

	t.Run("update of an application that was already processable keeps the local cache", func(t *testing.T) {
		name := run(t, func(h cache.ResourceEventHandlerFuncs, app *v1alpha1.Application) {
			old := app.DeepCopy()
			app.ResourceVersion = "2"
			h.OnUpdate(old, app)
		})
		// Unchanged behavior for apps that stay on this controller: the write is still deduplicated.
		assert.Equal(t, "deploy", name)
	})
}

func TestResyncAppsAfterShardChange_InvalidatesLocalCache(t *testing.T) {
	healthy := &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{Kind: "Pod", Name: "pod-1"}}}
	suspended := &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{Kind: "Deployment", Name: "deploy"}}}

	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	other, reader := withSharedTreeCache(ctrl)
	name := app.InstanceName(ctrl.namespace)

	// This controller wrote the tree, then the app's cluster moved to another shard, whose controller
	// overwrote it. The cluster is now back on this controller's shard, with no change to the app.
	require.NoError(t, ctrl.cache.SetAppResourcesTree(name, healthy))
	require.NoError(t, other.SetAppResourcesTree(name, suspended))
	require.NoError(t, ctrl.resyncAppsAfterShardChange())
	require.NoError(t, ctrl.cache.SetAppResourcesTree(name, healthy))

	got := &v1alpha1.ApplicationTree{}
	require.NoError(t, reader.GetAppResourcesTree(name, got))
	require.Len(t, got.Nodes, 1)
	assert.Equal(t, "pod-1", got.Nodes[0].Name)
	assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
}
