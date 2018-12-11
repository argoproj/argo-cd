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
	tags            []appv1.InfoItem
	appName         string
	resource        *unstructured.Unstructured
}

func (n *node) resourceKey() kube.ResourceKey {
	return kube.NewResourceKey(n.ref.GroupVersionKind().Group, n.ref.Kind, n.ref.Namespace, n.ref.Name)
}

func (n *node) isParentOf(child *node) bool {
	ownerGvk := n.ref.GroupVersionKind()
	for _, ownerRef := range child.ownerRefs {
		if kube.NewResourceKey(ownerGvk.Group, ownerRef.Kind, n.ref.Namespace, ownerRef.Name) == n.resourceKey() {
			return true
		}
	}

	return false
}

func ownerRefGV(ownerRef metav1.OwnerReference) schema.GroupVersion {
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return gv
}

func (n *node) getApp(ns map[kube.ResourceKey]*node) string {
	if n.appName != "" {
		return n.appName
	}
	for _, ownerRef := range n.ownerRefs {
		gv := ownerRefGV(ownerRef)
		if parent, ok := ns[kube.NewResourceKey(gv.Group, ownerRef.Kind, n.ref.Namespace, ownerRef.Name)]; ok {
			app := parent.getApp(ns)
			if app != "" {
				return app
			}
		}
	}
	return ""
}

func (n *node) childResourceNodes(ns map[kube.ResourceKey]*node) appv1.ResourceNode {
	children := make([]appv1.ResourceNode, 0)
	for key := range ns {
		if n.isParentOf(ns[key]) {
			children = append(children, ns[key].childResourceNodes(ns))
		}
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
