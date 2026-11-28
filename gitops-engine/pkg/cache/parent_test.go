package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

func customParentObject(name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-" + name))
	return u
}

func annotateParent(child, parent *unstructured.Unstructured) {
	child.SetAnnotations(map[string]string{
		AnnotationKeyParentAPIVersion: parent.GetAPIVersion(),
		AnnotationKeyParentKind:       parent.GetKind(),
		AnnotationKeyParentName:       parent.GetName(),
		AnnotationKeyParentNamespace:  parent.GetNamespace(),
	})
}

func TestParentReference(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		annotations map[string]string
		namespace   string
		want        bool
		wantErr     bool
	}{
		{name: "absent"},
		{name: "same namespace", annotations: map[string]string{AnnotationKeyParentAPIVersion: "apps/v1", AnnotationKeyParentKind: "Deployment", AnnotationKeyParentName: "parent"}, namespace: "child-ns", want: true},
		{name: "cross namespace", annotations: map[string]string{AnnotationKeyParentAPIVersion: "v1", AnnotationKeyParentKind: "ConfigMap", AnnotationKeyParentName: "parent", AnnotationKeyParentNamespace: "parent-ns"}, namespace: "parent-ns", want: true},
		{name: "cluster scoped", annotations: map[string]string{AnnotationKeyParentAPIVersion: "v1", AnnotationKeyParentKind: "Namespace", AnnotationKeyParentName: "parent", AnnotationKeyParentNamespace: ""}, want: true},
		{name: "incomplete", annotations: map[string]string{AnnotationKeyParentName: "parent"}, wantErr: true},
		{name: "namespace only", annotations: map[string]string{AnnotationKeyParentNamespace: ""}, wantErr: true},
		{name: "invalid api version", annotations: map[string]string{AnnotationKeyParentAPIVersion: "a/b/c", AnnotationKeyParentKind: "ConfigMap", AnnotationKeyParentName: "parent"}, wantErr: true},
		{name: "invalid version syntax", annotations: map[string]string{AnnotationKeyParentAPIVersion: "not a version", AnnotationKeyParentKind: "ConfigMap", AnnotationKeyParentName: "parent"}, wantErr: true},
		{name: "empty group prefix", annotations: map[string]string{AnnotationKeyParentAPIVersion: "/v1", AnnotationKeyParentKind: "ConfigMap", AnnotationKeyParentName: "parent"}, wantErr: true},
		{name: "self reference", annotations: map[string]string{AnnotationKeyParentAPIVersion: "v1", AnnotationKeyParentKind: "ConfigMap", AnnotationKeyParentName: "child"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := customParentObject("child", "child-ns")
			u.SetAnnotations(tc.annotations)
			before := u.DeepCopy()
			ref, err := parentReference(u)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.want {
				require.NotNil(t, ref)
				assert.Equal(t, tc.namespace, ref.Namespace)
				assert.Equal(t, tc.annotations[AnnotationKeyParentAPIVersion], ref.APIVersion)
				assert.Equal(t, tc.annotations[AnnotationKeyParentKind], ref.Kind)
				assert.Equal(t, tc.annotations[AnnotationKeyParentName], ref.Name)
			} else {
				assert.Nil(t, ref)
			}
			assert.Equal(t, before, u, "parsing must never mutate a Kubernetes object")
		})
	}
}

func TestCustomParentHierarchy(t *testing.T) {
	t.Parallel()
	for _, namespaces := range [][2]string{{"same", "same"}, {"parent-ns", "child-ns"}, {"", "child-ns"}, {"parent-ns", ""}} {
		t.Run(fmt.Sprintf("%q to %q", namespaces[0], namespaces[1]), func(t *testing.T) {
			t.Parallel()
			c := NewClusterCache(&rest.Config{})
			parent := customParentObject("parent", namespaces[0])
			child := customParentObject("child", namespaces[1])
			grandchild := customParentObject("grandchild", namespaces[1])
			annotateParent(child, parent)
			grandchild.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: child.GetName(), UID: child.GetUID()}})
			// A custom child can be observed before its parent; native descendants still work.
			c.setNode(c.newResource(child))
			c.setNode(c.newResource(grandchild))
			c.setNode(c.newResource(parent))
			keys := []kube.ResourceKey{}
			c.IterateHierarchyV2([]kube.ResourceKey{kube.GetResourceKey(parent)}, func(r *Resource, _ map[kube.ResourceKey]*Resource) bool {
				keys = append(keys, r.ResourceKey())
				return true
			})
			assert.ElementsMatch(t, []kube.ResourceKey{kube.GetResourceKey(parent), kube.GetResourceKey(child), kube.GetResourceKey(grandchild)}, keys)
			assert.Empty(t, c.resources[kube.GetResourceKey(child)].OwnerRefs)
			assert.Empty(t, child.GetOwnerReferences())
			// Parent identity, rather than a stale UID, survives recreation.
			c.onNodeRemoved(kube.GetResourceKey(parent))
			parent.SetUID("replacement")
			c.setNode(c.newResource(parent))
			assert.Len(t, getChildren(c, parent), 2)
			// Removing the annotation removes the cached edge immediately.
			oldChild := c.resources[kube.GetResourceKey(child)]
			child.SetAnnotations(nil)
			c.onNodeUpdated(oldChild, c.newResource(child))
			assert.Empty(t, getChildren(c, parent))
			assert.Empty(t, c.customParentToChildren)
		})
	}
}

func TestCustomParentCyclesAndUpdates(t *testing.T) {
	t.Parallel()
	c := NewClusterCache(&rest.Config{})
	a, b := customParentObject("a", "one"), customParentObject("b", "two")
	annotateParent(a, b)
	annotateParent(b, a)
	c.setNode(c.newResource(a))
	c.setNode(c.newResource(b))
	var visited []kube.ResourceKey
	c.IterateHierarchyV2([]kube.ResourceKey{kube.GetResourceKey(a), kube.GetResourceKey(b)}, func(r *Resource, _ map[kube.ResourceKey]*Resource) bool {
		visited = append(visited, r.ResourceKey())
		return true
	})
	assert.Len(t, visited, 2, "cycles and overlapping roots must visit each resource once")
	visited = nil
	c.IterateHierarchyV2([]kube.ResourceKey{kube.GetResourceKey(a)}, func(r *Resource, _ map[kube.ResourceKey]*Resource) bool {
		visited = append(visited, r.ResourceKey())
		return false
	})
	assert.Equal(t, []kube.ResourceKey{kube.GetResourceKey(a)}, visited)
	// Reparenting cleans up the old edge, even if the new parent is not cached yet.
	other := customParentObject("other", "three")
	old := c.resources[kube.GetResourceKey(b)]
	annotateParent(b, other)
	c.onNodeUpdated(old, c.newResource(b))
	assert.NotContains(t, c.customParentToChildren, kube.GetResourceKey(a))
	assert.Contains(t, c.customParentToChildren[kube.GetResourceKey(other)], kube.GetResourceKey(b))
	c.onNodeRemoved(kube.GetResourceKey(b))
	assert.NotContains(t, c.customParentToChildren, kube.GetResourceKey(other))
}

func TestCustomParentInitialSyncAndResync(t *testing.T) {
	t.Parallel()
	parent, child := customParentObject("parent", "default"), customParentObject("child", "default")
	parent.SetKind("Pod")
	child.SetKind("Pod")
	annotateParent(child, parent)
	c := newCluster(t, child, parent)
	for range 2 {
		require.NoError(t, c.EnsureSynced())
		assert.Len(t, getChildren(c, parent), 1)
		assert.Len(t, c.customParentToChildren, 1)
		c.Invalidate()
	}
}

func TestCustomParentInvalidReferencePreservesOwners(t *testing.T) {
	t.Parallel()
	c := NewClusterCache(&rest.Config{})
	u := customParentObject("child", "default")
	u.SetAnnotations(map[string]string{AnnotationKeyParentName: "incomplete"})
	u.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "native", UID: "native-uid"}})
	r := c.newResource(u)
	assert.Nil(t, r.ParentRef)
	assert.Equal(t, u.GetOwnerReferences(), r.OwnerRefs)
}

func TestCustomParentUpdateNotifications(t *testing.T) {
	t.Parallel()
	c := NewClusterCache(&rest.Config{})
	parent, child := customParentObject("parent", "one"), customParentObject("child", "two")
	annotateParent(child, parent)
	c.setNode(c.newResource(parent))
	c.setNode(c.newResource(child))
	called := false
	_ = c.OnResourceUpdated(func(newRes, oldRes *Resource, resources map[kube.ResourceKey]*Resource) {
		called = true
		assert.Contains(t, resources, kube.GetResourceKey(parent), "refresh callbacks need access to cross-namespace ancestors")
		assert.NotNil(t, oldRes.ParentRef)
		assert.Nil(t, newRes.ParentRef)
	})
	old := c.resources[kube.GetResourceKey(child)]
	child.SetAnnotations(nil)
	c.onNodeUpdated(old, c.newResource(child))
	assert.True(t, called)
}
