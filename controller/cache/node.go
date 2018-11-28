package cache

import (
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type node struct {
	resourceVersion string
	ref             v1.ObjectReference
	ownerRefs       []metav1.OwnerReference
	children        map[kube.ResourceKey]*node
	parents         map[kube.ResourceKey]*node
	tags            []string
	appName         string
	resource        *unstructured.Unstructured
}

func (n *node) resourceKey() kube.ResourceKey {
	return kube.NewResourceKey(n.ref.GroupVersionKind().Group, n.ref.Kind, n.ref.Namespace, n.ref.Name)
}

func (n *node) isParentOf(child *node) bool {
	if n.ref.Namespace != child.ref.Namespace {
		return false
	}
	ownerGvk := n.ref.GroupVersionKind()
	for _, ownerRef := range child.ownerRefs {
		if kube.NewResourceKey(ownerGvk.Group, ownerRef.Kind, n.ref.Namespace, ownerRef.Name) == n.resourceKey() {
			return true
		}
	}

	return false
}

func (n *node) setAppName(appName string) {
	n.appName = appName
	for i := range n.children {
		n.children[i].setAppName(appName)
	}
}

func (n *node) fillChildren(nodes map[kube.ResourceKey]*node) {
	for k, child := range nodes {
		if n.isParentOf(child) {
			delete(nodes, k)
			child.appName = n.appName
			child.parents[n.resourceKey()] = n
			n.children[child.resourceKey()] = child
			child.fillChildren(nodes)
		}
	}
}

func (n *node) childResourceNodes() appv1.ResourceNode {
	children := make([]appv1.ResourceNode, 0)
	for i := range n.children {
		children = append(children, n.children[i].childResourceNodes())
	}
	gv, err := schema.ParseGroupVersion(n.ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return appv1.ResourceNode{
		Name:            n.ref.Name,
		Group:           gv.Group,
		Version:         gv.Version,
		Kind:            n.ref.Kind,
		Namespace:       n.ref.Namespace,
		Tags:            n.tags,
		Children:        children,
		ResourceVersion: n.resourceVersion,
	}
}
