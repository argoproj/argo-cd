package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func newTestSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
}

// recorder captures the callbacks passed to secretEventHandlerFuncs so tests can
// assert which handler fired and with which object(s).
type recorder struct {
	added    []*corev1.Secret
	deleted  []*corev1.Secret
	modOld   []*corev1.Secret
	modNew   []*corev1.Secret
	addCount int
	delCount int
	modCount int
}

func (r *recorder) handler() cache.ResourceEventHandlerFuncs {
	return secretEventHandlerFuncs(
		func(s *corev1.Secret) {
			r.addCount++
			r.added = append(r.added, s)
		},
		func(oldS, newS *corev1.Secret) {
			r.modCount++
			r.modOld = append(r.modOld, oldS)
			r.modNew = append(r.modNew, newS)
		},
		func(s *corev1.Secret) {
			r.delCount++
			r.deleted = append(r.deleted, s)
		},
	)
}

func TestSecretEventHandlerFuncs_AddFunc(t *testing.T) {
	t.Run("secret is forwarded to the add callback", func(t *testing.T) {
		r := &recorder{}
		secret := newTestSecret("repo-secret")

		r.handler().OnAdd(secret, false)

		require.Equal(t, 1, r.addCount)
		assert.Same(t, secret, r.added[0])
	})

	t.Run("unexpected object type is ignored", func(t *testing.T) {
		r := &recorder{}

		assert.NotPanics(t, func() {
			r.handler().OnAdd(&corev1.ConfigMap{}, false)
			r.handler().OnAdd("not-a-secret", false)
			r.handler().OnAdd(nil, false)
		})
		assert.Zero(t, r.addCount)
	})
}

func TestSecretEventHandlerFuncs_UpdateFunc(t *testing.T) {
	t.Run("both secrets are forwarded to the mod callback", func(t *testing.T) {
		r := &recorder{}
		oldSecret := newTestSecret("repo-secret")
		newSecret := newTestSecret("repo-secret")
		newSecret.ResourceVersion = "2"

		r.handler().OnUpdate(oldSecret, newSecret)

		require.Equal(t, 1, r.modCount)
		assert.Same(t, oldSecret, r.modOld[0])
		assert.Same(t, newSecret, r.modNew[0])
	})

	t.Run("ignored when old object is not a secret", func(t *testing.T) {
		r := &recorder{}

		assert.NotPanics(t, func() {
			r.handler().OnUpdate("not-a-secret", newTestSecret("repo-secret"))
		})
		assert.Zero(t, r.modCount)
	})

	t.Run("ignored when new object is not a secret", func(t *testing.T) {
		r := &recorder{}

		assert.NotPanics(t, func() {
			r.handler().OnUpdate(newTestSecret("repo-secret"), "not-a-secret")
		})
		assert.Zero(t, r.modCount)
	})
}

func TestSecretEventHandlerFuncs_DeleteFunc(t *testing.T) {
	t.Run("secret is forwarded to the delete callback", func(t *testing.T) {
		r := &recorder{}
		secret := newTestSecret("repo-secret")

		r.handler().OnDelete(secret)

		require.Equal(t, 1, r.delCount)
		assert.Same(t, secret, r.deleted[0])
	})

	t.Run("tombstone wrapping a secret is unwrapped and forwarded", func(t *testing.T) {
		r := &recorder{}
		secret := newTestSecret("repo-secret")
		tombstone := cache.DeletedFinalStateUnknown{Key: testNamespace + "/repo-secret", Obj: secret}

		r.handler().OnDelete(tombstone)

		require.Equal(t, 1, r.delCount)
		assert.Same(t, secret, r.deleted[0])
	})

	t.Run("tombstone wrapping a non-secret is ignored without panicking", func(t *testing.T) {
		r := &recorder{}
		tombstone := cache.DeletedFinalStateUnknown{Key: "ns/cm", Obj: &corev1.ConfigMap{}}

		assert.NotPanics(t, func() {
			r.handler().OnDelete(tombstone)
		})
		assert.Zero(t, r.delCount)
	})

	t.Run("tombstone wrapping nil is ignored without panicking", func(t *testing.T) {
		r := &recorder{}
		tombstone := cache.DeletedFinalStateUnknown{Key: "ns/gone", Obj: nil}

		assert.NotPanics(t, func() {
			r.handler().OnDelete(tombstone)
		})
		assert.Zero(t, r.delCount)
	})

	t.Run("unexpected object type is ignored without panicking", func(t *testing.T) {
		r := &recorder{}

		assert.NotPanics(t, func() {
			r.handler().OnDelete("not-a-secret")
			r.handler().OnDelete(nil)
		})
		assert.Zero(t, r.delCount)
	})
}
