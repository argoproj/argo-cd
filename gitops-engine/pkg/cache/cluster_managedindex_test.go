package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

type indexedApp struct {
	Name string
}

func indexByInstanceLabel(un *unstructured.Unstructured, isRoot bool) (any, bool) {
	name := ""
	if isRoot {
		name = un.GetLabels()["app.kubernetes.io/instance"]
	}
	return &indexedApp{Name: name}, true
}

func indexedAppName(r *Resource) string {
	info, ok := r.Info.(*indexedApp)
	if !ok || info == nil {
		return ""
	}
	return info.Name
}

func TestGetManagedLiveObjsForKeyMatchesFullScan(t *testing.T) {
	t.Parallel()
	other := testDeploy()
	other.Name = "other"
	other.UID = "99"
	other.Labels = map[string]string{"app.kubernetes.io/instance": "other-app"}

	cluster := newCluster(t, testPod1(), testRS(), testDeploy(), other)
	cluster.Invalidate(
		SetPopulateResourceInfoHandler(indexByInstanceLabel),
		SetManagedResourceKey(indexedAppName),
	)
	require.NoError(t, cluster.EnsureSynced())

	target := mustToUnstructured(testDeploy())
	predicate := func(r *Resource) bool { return indexedAppName(r) == "helm-guestbook" }

	full, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{target}, predicate)
	require.NoError(t, err)
	indexed, err := cluster.GetManagedLiveObjsForKey("helm-guestbook", []*unstructured.Unstructured{target})
	require.NoError(t, err)
	assert.Equal(t, full, indexed)

	_, leaked := indexed[kube.NewResourceKey("apps", "Deployment", "default", "other")]
	assert.False(t, leaked)
	_, found := indexed[kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook")]
	assert.True(t, found)

	cluster.namespaces = []string{"default"}
	_, err = cluster.GetManagedLiveObjsForKey("helm-guestbook", []*unstructured.Unstructured{{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "outside",
				"namespace": "not-managed",
			},
		},
	}})
	require.Error(t, err)
}

func TestManagedIndexFollowsReplaceAndDelete(t *testing.T) {
	t.Parallel()
	cluster := newCluster(t, testDeploy())
	cluster.Invalidate(
		SetPopulateResourceInfoHandler(indexByInstanceLabel),
		SetManagedResourceKey(indexedAppName),
	)
	require.NoError(t, cluster.EnsureSynced())

	deployKey := kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook")
	cluster.lock.RLock()
	_, indexed := cluster.managedIndex["helm-guestbook"][deployKey]
	cluster.lock.RUnlock()
	require.True(t, indexed)

	moved := testDeploy()
	moved.ResourceVersion = "124"
	moved.Labels = map[string]string{"app.kubernetes.io/instance": "moved"}
	cluster.lock.Lock()
	cluster.setNode(cluster.newResource(mustToUnstructured(moved)))
	cluster.lock.Unlock()

	cluster.lock.RLock()
	_, stillOld := cluster.managedIndex["helm-guestbook"][deployKey]
	_, nowMoved := cluster.managedIndex["moved"][deployKey]
	cluster.lock.RUnlock()
	assert.False(t, stillOld)
	assert.True(t, nowMoved)

	cluster.lock.Lock()
	cluster.onNodeRemoved(deployKey)
	cluster.lock.Unlock()

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	assert.Empty(t, cluster.managedIndex["moved"])
	_, stillThere := cluster.resources[deployKey]
	assert.False(t, stillThere)
}

func TestGetManagedLiveObjsForKeyRequiresIndex(t *testing.T) {
	t.Parallel()
	cluster := newCluster(t, testDeploy())
	require.NoError(t, cluster.EnsureSynced())
	_, err := cluster.GetManagedLiveObjsForKey("helm-guestbook", nil)
	require.ErrorIs(t, err, ErrManagedIndexNotConfigured)
}
