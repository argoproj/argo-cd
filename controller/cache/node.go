package cache

import (
	log "github.com/sirupsen/logrus"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type node struct {
	resourceVersion string
	ref             v1.ObjectReference
	ownerRefs       []metav1.OwnerReference
	info            []appv1.InfoItem
	appName         string
	resource        *unstructured.Unstructured
}

func (n *node) resourceKey() kube.ResourceKey {
	return kube.NewResourceKey(n.ref.GroupVersionKind().Group, n.ref.Kind, n.ref.Namespace, n.ref.Name)
}

func (n *node) isParentOf(child *node) bool {
	for _, ownerRef := range child.ownerRefs {
		ownerGvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
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

func newResourceKeySet(set map[kube.ResourceKey]bool, keys ...kube.ResourceKey) map[kube.ResourceKey]bool {
	newSet := make(map[kube.ResourceKey]bool)
	for k, v := range set {
		newSet[k] = v
	}
	for i := range keys {
		newSet[keys[i]] = true
	}
	return newSet
}

func (n *node) childResourceNodes(ns map[kube.ResourceKey]*node, parents map[kube.ResourceKey]bool) appv1.ResourceNode {
	children := make([]appv1.ResourceNode, 0)
	for childKey := range ns {
		if n.isParentOf(ns[childKey]) {
			if parents[childKey] {
				key := n.resourceKey()
				log.Warnf("Circular dependency detected. %s is child and parent of %s", childKey.String(), key.String())
			} else {
				children = append(children, ns[childKey].childResourceNodes(ns, newResourceKeySet(parents, n.resourceKey())))
			}
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
		Info:            n.info,
		Children:        children,
		ResourceVersion: n.resourceVersion,
	}
}
