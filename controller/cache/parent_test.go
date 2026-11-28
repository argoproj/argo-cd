package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestCustomParentResourceNode(t *testing.T) {
	t.Parallel()
	parent := &clustercache.Resource{
		Ref:  corev1.ObjectReference{APIVersion: "example.com/v1", Kind: "Parent", Namespace: "parent-ns", Name: "parent", UID: "parent-uid"},
		Info: &ResourceInfo{AppName: "parent-app"},
	}
	child := &clustercache.Resource{
		Ref:       corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "child-ns", Name: "child", UID: "child-uid"},
		ParentRef: &corev1.ObjectReference{APIVersion: "example.com/v1", Kind: "Parent", Namespace: "parent-ns", Name: "parent"},
		OwnerRefs: []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "native-owner", UID: "native-uid"}},
		Info:      &ResourceInfo{},
	}
	resources := map[kube.ResourceKey]*clustercache.Resource{parent.ResourceKey(): parent, child.ResourceKey(): child}
	node := asResourceNode(child, resources)
	require.Len(t, node.ParentRefs, 2, "custom relationships must preserve native ownership")
	assert.Equal(t, appv1.ResourceRef{Group: "example.com", Version: "v1", Kind: "Parent", Namespace: "parent-ns", Name: "parent", UID: "parent-uid"}, node.ParentRefs[1])
	assert.Equal(t, "parent-app", getApp(child, resources), "child changes must refresh the parent's app")
	assert.False(t, isRootAppNode(child), "a display link must not confer sync/prune ownership")
	native := &clustercache.Resource{
		Ref:  corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "child-ns", Name: "native-owner", UID: "native-uid"},
		Info: &ResourceInfo{AppName: "native-app"},
	}
	resources[native.ResourceKey()] = native
	assert.Equal(t, "native-app", getApp(child, resources), "native ownership retains its existing attribution")
	child.Info = &ResourceInfo{AppName: "child-app"}
	assert.Equal(t, "child-app", getApp(child, resources), "explicit application tracking takes precedence")
	delete(resources, parent.ResourceKey())
	node = asResourceNode(child, resources)
	assert.Empty(t, node.ParentRefs[1].UID, "missing parents must not borrow another object's UID")
}

func TestCustomParentResourceNodeDeduplicatesOwner(t *testing.T) {
	t.Parallel()
	parent := &clustercache.Resource{Ref: corev1.ObjectReference{APIVersion: "example.com/v2", Kind: "Parent", Namespace: "default", Name: "parent", UID: "parent-uid"}}
	child := &clustercache.Resource{
		Ref:       corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "child"},
		ParentRef: parent.Ref.DeepCopy(),
		OwnerRefs: []metav1.OwnerReference{{APIVersion: "example.com/v1", Kind: "Parent", Name: "parent", UID: "parent-uid"}},
	}
	node := asResourceNode(child, map[kube.ResourceKey]*clustercache.Resource{parent.ResourceKey(): parent})
	assert.Len(t, node.ParentRefs, 1)
}

func TestCustomParentGetAppCycle(t *testing.T) {
	t.Parallel()
	a := &clustercache.Resource{Ref: corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "a"}, Info: &ResourceInfo{}}
	b := &clustercache.Resource{Ref: corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "b"}, Info: &ResourceInfo{}}
	a.ParentRef = b.Ref.DeepCopy()
	b.ParentRef = a.Ref.DeepCopy()
	assert.Empty(t, getApp(a, map[kube.ResourceKey]*clustercache.Resource{a.ResourceKey(): a, b.ResourceKey(): b}))
}
