package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
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
	return &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: test.FakeArgoCDNamespace}}
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

	t.Run("update of a processable application requeues a refresh and an operation", func(t *testing.T) {
		ctrl := newFakeController(t.Context(), &fakeData{}, nil)
		h := ctrl.applicationEventHandlerFuncs()

		old := newFakeApp()
		updated := newFakeApp()
		updated.ResourceVersion = "2"
		h.OnUpdate(old, updated)

		assert.Contains(t, assertEnqueued(t, ctrl.appRefreshQueue), "my-app")
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
