package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

// newTestEnforcer returns an Enforcer backed by a fake clientset, suitable for
// exercising the informer event handlers in isolation.
func newTestEnforcer() *Enforcer {
	return NewEnforcer(fake.NewClientset(), fakeNamespace, fakeConfigMapName, nil)
}

func TestRBACConfigMapEventHandler_AddFunc(t *testing.T) {
	t.Run("valid ConfigMap is synced and the update callback is invoked", func(t *testing.T) {
		enf := newTestEnforcer()
		cm := fakeConfigMap()
		cm.Data[ConfigMapPolicyDefaultKey] = "role:readonly"

		var updated *corev1.ConfigMap
		handler := enf.rbacConfigMapEventHandler(func(cm *corev1.ConfigMap) error {
			updated = cm
			return nil
		})

		handler.OnAdd(cm, false)

		require.NotNil(t, updated, "update callback should have been invoked")
		assert.Equal(t, fakeConfigMapName, updated.Name)
		// syncUpdate sets the default role from the ConfigMap.
		assert.Equal(t, "role:readonly", enf.defaultRole)
	})

	t.Run("unexpected object type is ignored without panicking", func(t *testing.T) {
		enf := newTestEnforcer()
		called := false
		handler := enf.rbacConfigMapEventHandler(func(_ *corev1.ConfigMap) error {
			called = true
			return nil
		})

		assert.NotPanics(t, func() {
			handler.OnAdd("not-a-configmap", false)
			handler.OnAdd(&corev1.Secret{}, false)
		})
		assert.False(t, called, "update callback must not be invoked for unexpected types")
	})
}

func TestRBACConfigMapEventHandler_UpdateFunc(t *testing.T) {
	t.Run("resource version change triggers a sync with the new ConfigMap", func(t *testing.T) {
		enf := newTestEnforcer()
		oldCM := fakeConfigMap()
		oldCM.ResourceVersion = "1"
		newCM := fakeConfigMap()
		newCM.ResourceVersion = "2"
		newCM.Data[ConfigMapPolicyDefaultKey] = "role:admin"

		var updated *corev1.ConfigMap
		handler := enf.rbacConfigMapEventHandler(func(cm *corev1.ConfigMap) error {
			updated = cm
			return nil
		})

		handler.OnUpdate(oldCM, newCM)

		require.NotNil(t, updated)
		assert.Equal(t, "2", updated.ResourceVersion)
		assert.Equal(t, "role:admin", enf.defaultRole)
	})

	t.Run("unchanged resource version is skipped", func(t *testing.T) {
		enf := newTestEnforcer()
		oldCM := fakeConfigMap()
		oldCM.ResourceVersion = "7"
		newCM := fakeConfigMap()
		newCM.ResourceVersion = "7"

		called := false
		handler := enf.rbacConfigMapEventHandler(func(_ *corev1.ConfigMap) error {
			called = true
			return nil
		})

		handler.OnUpdate(oldCM, newCM)

		assert.False(t, called, "sync must be skipped when the resource version is unchanged")
	})

	t.Run("unexpected object types are ignored without panicking", func(t *testing.T) {
		enf := newTestEnforcer()
		called := false
		handler := enf.rbacConfigMapEventHandler(func(_ *corev1.ConfigMap) error {
			called = true
			return nil
		})

		cm := fakeConfigMap()
		tombstone := cache.DeletedFinalStateUnknown{Key: "fake-ns/fake-cm", Obj: fakeConfigMap()}

		assert.NotPanics(t, func() {
			// old is not a ConfigMap
			handler.OnUpdate("not-a-configmap", cm)
			// new is not a ConfigMap
			handler.OnUpdate(cm, "not-a-configmap")
			// both unexpected
			handler.OnUpdate(nil, nil)
			// tombstones are not *corev1.ConfigMap and must not panic
			handler.OnUpdate(tombstone, tombstone)
		})
		assert.False(t, called, "update callback must not be invoked for unexpected types")
	})
}
