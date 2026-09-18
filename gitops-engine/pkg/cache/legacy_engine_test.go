package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// TestHandleCRDEvent_LockContract checks that handleCRDEvent, called without
// c.lock, takes the lock itself around startMissingWatches and
// reloadOpenAPISchema. If either callee took it too, the non-reentrant lock
// would deadlock and the test would time out.
func TestHandleCRDEvent_LockContract(t *testing.T) {
	t.Parallel()
	cluster := newCluster(t)
	require.NoError(t, cluster.EnsureSynced())

	crd := strToUnstructured(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: dummies.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    kind: Dummy
    plural: dummies
    singular: dummy
  versions:
  - name: v1`)
	require.True(t, kube.IsCRD(crd), "sanity: the fixture must be recognised as a CRD")

	// All three event types take the same locking path as the watch loop.
	for _, event := range []watch.EventType{watch.Added, watch.Modified, watch.Deleted} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			cluster.handleCRDEvent(legacyEngineOf(cluster), event, crd)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("handleCRDEvent(%s) did not return; store.lock is likely acquired twice", event)
		}
	}

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	for _, api := range cluster.GetAPIResources() {
		assert.NotEqual(t, "Dummy", api.GroupKind.Kind, "the deleted CRD must not remain in apiResources")
	}
}
