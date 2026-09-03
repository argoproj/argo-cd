package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// instanceKeyFn groups cached resources by their argo-style instance label,
// mirroring how the application controller keys resources by owning app.
func instanceKeyFn(r *Resource) string {
	if r.Resource == nil {
		return ""
	}
	return r.Resource.GetLabels()["app.kubernetes.io/instance"]
}

// buildSyncedIndexedCache builds a cluster cache that caches manifests and keeps
// a managed index keyed by the instance label, then syncs it.
func buildSyncedIndexedCache(t testing.TB) *clusterCache {
	t.Helper()
	c := newClusterWithOptions(t, []UpdateSettingsFunc{
		SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (any, bool) { return nil, true }),
		SetManagedIndexKeyFn(instanceKeyFn),
	}, testDeploy(), testRS(), testPod1())
	require.NoError(t, c.EnsureSynced())
	return c
}

func guestbookTarget() *unstructured.Unstructured {
	return strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  namespace: default
  labels:
    app.kubernetes.io/instance: helm-guestbook`)
}

// TestGetManagedLiveObjsForKey_MatchesScan asserts the indexed lookup returns
// exactly the same result as the full-scan predicate variant.
func TestGetManagedLiveObjsForKey_MatchesScan(t *testing.T) {
	c := buildSyncedIndexedCache(t)
	targets := []*unstructured.Unstructured{guestbookTarget()}

	scan, err := c.GetManagedLiveObjs(targets, func(r *Resource) bool {
		return instanceKeyFn(r) == "helm-guestbook"
	})
	require.NoError(t, err)

	idx, err := c.GetManagedLiveObjsForKey(targets, "helm-guestbook")
	require.NoError(t, err)

	assert.Equal(t, scan, idx, "indexed lookup must equal full scan")
	assert.Contains(t, idx, kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"))
}

// TestGetManagedLiveObjsForKey_UnknownKey returns an empty managed set for an
// unindexed key, never a panic.
func TestGetManagedLiveObjsForKey_UnknownKey(t *testing.T) {
	c := buildSyncedIndexedCache(t)
	idx, err := c.GetManagedLiveObjsForKey([]*unstructured.Unstructured{}, "does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, idx)
}

// TestManagedIndexEquivalenceAfterResync ensures the index is rebuilt correctly
// after Invalidate()+EnsureSynced (exercises the reset path).
func TestManagedIndexEquivalenceAfterResync(t *testing.T) {
	c := buildSyncedIndexedCache(t)
	c.Invalidate()
	require.NoError(t, c.EnsureSynced())

	targets := []*unstructured.Unstructured{guestbookTarget()}
	scan, err := c.GetManagedLiveObjs(targets, func(r *Resource) bool {
		return instanceKeyFn(r) == "helm-guestbook"
	})
	require.NoError(t, err)
	idx, err := c.GetManagedLiveObjsForKey(targets, "helm-guestbook")
	require.NoError(t, err)
	assert.Equal(t, scan, idx)
}

// TestUpdateManagedIndex is a white-box test of index maintenance across the
// add / replace(key change) / remove transitions that setNode and onNodeRemoved
// drive in production.
func TestUpdateManagedIndex(t *testing.T) {
	c := &clusterCache{managedIndexKeyFn: instanceKeyFn}

	mk := func(name, instance string) *Resource {
		un := strToUnstructured(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: ` + name + `
  namespace: default
  labels:
    app.kubernetes.io/instance: ` + instance)
		return &Resource{
			Ref:      corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: name, Namespace: "default"},
			Resource: un,
		}
	}

	a := mk("cm-a", "app-1")

	// add
	c.updateManagedIndex(nil, a)
	assert.Len(t, c.managedIndex["app-1"], 1)
	assert.Contains(t, c.managedIndex["app-1"], a.ResourceKey())

	// replace with the app label changed -> must move buckets, pruning the old
	aMoved := mk("cm-a", "app-2")
	c.updateManagedIndex(a, aMoved)
	assert.NotContains(t, c.managedIndex, "app-1", "empty bucket must be pruned")
	assert.Len(t, c.managedIndex["app-2"], 1)
	assert.Contains(t, c.managedIndex["app-2"], aMoved.ResourceKey())

	// remove
	c.updateManagedIndex(aMoved, nil)
	assert.NotContains(t, c.managedIndex, "app-2")

	// unmanaged resource (empty key) is never indexed
	c.updateManagedIndex(nil, mk("cm-b", ""))
	assert.Empty(t, c.managedIndex)
}

// TestUpdateManagedIndex_NoKeyFn is a no-op safety check: with no key function
// configured the index stays nil.
func TestUpdateManagedIndex_NoKeyFn(t *testing.T) {
	c := &clusterCache{}
	c.updateManagedIndex(nil, &Resource{Ref: corev1.ObjectReference{Kind: "ConfigMap", Name: "x", Namespace: "default"}})
	assert.Nil(t, c.managedIndex)
}
