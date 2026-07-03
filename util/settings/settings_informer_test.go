package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
)

// newChangeTrackingManager returns a SettingsManager whose onRepoOrClusterChanged
// callback signals the returned channel. onRepoOrClusterChanged dispatches the
// callback in a goroutine, so use waitFired/assertNotFired to observe it.
func newChangeTrackingManager(t *testing.T) (*SettingsManager, <-chan struct{}) {
	t.Helper()
	fired := make(chan struct{}, 16)
	mgr := NewSettingsManager(t.Context(), fake.NewClientset(), "argocd", WithRepoOrClusterChangedHandler(func() {
		fired <- struct{}{}
	}))
	return mgr, fired
}

func waitFired(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("expected onRepoOrClusterChanged to fire, but it did not")
	}
}

func assertNotFired(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("onRepoOrClusterChanged fired but was not expected to")
	case <-time.After(200 * time.Millisecond):
	}
}

func argoCDConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: common.ArgoCDConfigMapName, Namespace: "argocd"}}
}

func repositorySecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      "repo-secret",
		Namespace: "argocd",
		Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
	}}
}

func TestArgoCDConfigMapEventHandler(t *testing.T) {
	t.Run("add/update/delete of argocd-cm triggers a change", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.argoCDConfigMapEventHandler()

		h.OnAdd(argoCDConfigMap(), false)
		waitFired(t, fired)

		h.OnUpdate(argoCDConfigMap(), argoCDConfigMap())
		waitFired(t, fired)

		h.OnDelete(argoCDConfigMap())
		waitFired(t, fired)
	})

	t.Run("a different configmap does not trigger a change", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.argoCDConfigMapEventHandler()

		other := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-rbac-cm", Namespace: "argocd"}}
		h.OnAdd(other, false)
		h.OnUpdate(other, other)
		h.OnDelete(other)
		assertNotFired(t, fired)
	})

	t.Run("delete unwraps a tombstone wrapping argocd-cm", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.argoCDConfigMapEventHandler()

		h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/argocd-cm", Obj: argoCDConfigMap()})
		waitFired(t, fired)
	})

	t.Run("delete of a tombstone wrapping nil or an unexpected type is ignored", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.argoCDConfigMapEventHandler()

		assert.NotPanics(t, func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/gone", Obj: nil})
			h.OnDelete("not-a-configmap")
			h.OnAdd("not-a-configmap", false)
		})
		assertNotFired(t, fired)
	})
}

func TestRepositorySecretEventHandler(t *testing.T) {
	t.Run("add/update/delete of a repository secret triggers a change", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.repositorySecretEventHandler()

		h.OnAdd(repositorySecret(), false)
		waitFired(t, fired)

		h.OnUpdate(repositorySecret(), repositorySecret())
		waitFired(t, fired)

		h.OnDelete(repositorySecret())
		waitFired(t, fired)
	})

	t.Run("a non-repository secret does not trigger a change", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.repositorySecretEventHandler()

		plain := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "plain", Namespace: "argocd"}}
		h.OnAdd(plain, false)
		h.OnUpdate(plain, plain)
		h.OnDelete(plain)
		assertNotFired(t, fired)
	})

	t.Run("delete unwraps a tombstone wrapping a repository secret", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.repositorySecretEventHandler()

		h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/repo-secret", Obj: repositorySecret()})
		waitFired(t, fired)
	})

	t.Run("tombstone wrapping a non-repository object or unexpected types are ignored", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.repositorySecretEventHandler()

		assert.NotPanics(t, func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/cm", Obj: &corev1.ConfigMap{}})
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/gone", Obj: nil})
			h.OnAdd("not-a-secret", false)
		})
		assertNotFired(t, fired)
	})
}

func TestClusterSecretEventHandler(t *testing.T) {
	// The cluster informer is pre-filtered to secret-type=cluster, so every event is
	// expected to trigger a change unconditionally.
	t.Run("add/update/delete always triggers a change", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.clusterSecretEventHandler()

		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-secret",
			Namespace: "argocd",
			Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeCluster},
		}}

		h.OnAdd(secret, false)
		waitFired(t, fired)

		h.OnUpdate(secret, secret)
		waitFired(t, fired)

		h.OnDelete(secret)
		waitFired(t, fired)
	})

	t.Run("delete unwraps a tombstone and still triggers a change without panicking", func(t *testing.T) {
		mgr, fired := newChangeTrackingManager(t)
		h := mgr.clusterSecretEventHandler()

		assert.NotPanics(t, func() {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "argocd/gone", Obj: nil})
		})
		waitFired(t, fired)
	})
}

func TestSettingsNotificationEventHandler(t *testing.T) {
	const partOfLabel = "app.kubernetes.io/part-of"
	settingsObject := func(creation time.Time, resourceVersion string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:              "argocd-secret",
			Namespace:         "argocd",
			Labels:            map[string]string{partOfLabel: "argocd"},
			CreationTimestamp: metav1.NewTime(creation),
			ResourceVersion:   resourceVersion,
		}}
	}

	t.Run("add of a settings object created after the cutoff notifies", func(t *testing.T) {
		now := time.Now()
		count := 0
		h := settingsNotificationEventHandler(now, func() { count++ })

		h.OnAdd(settingsObject(now.Add(time.Hour), "1"), false)
		assert.Equal(t, 1, count)
	})

	t.Run("add of a settings object created before the cutoff does not notify", func(t *testing.T) {
		now := time.Now()
		count := 0
		h := settingsNotificationEventHandler(now, func() { count++ })

		h.OnAdd(settingsObject(now.Add(-time.Hour), "1"), false)
		assert.Zero(t, count)
	})

	t.Run("add of a non-settings object or unexpected type does not notify", func(t *testing.T) {
		now := time.Now()
		count := 0
		h := settingsNotificationEventHandler(now, func() { count++ })

		nonSettings := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "plain", CreationTimestamp: metav1.NewTime(now.Add(time.Hour))}}
		assert.NotPanics(t, func() {
			h.OnAdd(nonSettings, false)
			h.OnAdd("not-an-object", false)
		})
		assert.Zero(t, count)
	})

	t.Run("update notifies only when the resource version changes", func(t *testing.T) {
		now := time.Now()
		count := 0
		h := settingsNotificationEventHandler(now, func() { count++ })

		h.OnUpdate(settingsObject(now, "1"), settingsObject(now, "2"))
		assert.Equal(t, 1, count)

		h.OnUpdate(settingsObject(now, "2"), settingsObject(now, "2"))
		assert.Equal(t, 1, count, "unchanged resource version must not notify")
	})

	t.Run("update of a non-settings object or unexpected type does not notify", func(t *testing.T) {
		now := time.Now()
		count := 0
		h := settingsNotificationEventHandler(now, func() { count++ })

		nonSettings := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "plain", ResourceVersion: "2"}}
		assert.NotPanics(t, func() {
			// new object is not a settings object
			h.OnUpdate(settingsObject(now, "1"), nonSettings)
			// old object is not a typed metadata object
			h.OnUpdate("not-an-object", settingsObject(now, "2"))
		})
		assert.Zero(t, count)
	})
}
