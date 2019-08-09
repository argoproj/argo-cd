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
	// available only for root application nodes
	resource *unstructured.Unstructured
	// networkingInfo are available only for known types involved into networking: Ingress, Service, Pod
	networkingInfo *appv1.ResourceNetworkingInfo
	images         []string
	health         *appv1.HealthStatus
}

func (n *node) isRootAppNode() bool {
	return n.appName != "" && len(n.ownerRefs) == 0
}

func (n *node) resourceKey() kube.ResourceKey {
	return kube.NewResourceKey(n.ref.GroupVersionKind().Group, n.ref.Kind, n.ref.Namespace, n.ref.Name)
}

func (n *node) isParentOf(child *node) bool {
	for i, ownerRef := range child.ownerRefs {

		// backfill UID of inferred owner child references
		if ownerRef.UID == "" && n.ref.Kind == ownerRef.Kind && n.ref.APIVersion == ownerRef.APIVersion && n.ref.Name == ownerRef.Name {
			ownerRef.UID = n.ref.UID
			child.ownerRefs[i] = ownerRef
			return true
		}

		if n.ref.UID == ownerRef.UID {
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
	return n.getAppRecursive(ns, map[kube.ResourceKey]bool{})
}

func (n *node) getAppRecursive(ns map[kube.ResourceKey]*node, visited map[kube.ResourceKey]bool) string {
	if !visited[n.resourceKey()] {
		visited[n.resourceKey()] = true
	} else {
		log.Warnf("Circular dependency detected: %v.", visited)
		return n.appName
	}

	if n.appName != "" {
		return n.appName
	}
	for _, ownerRef := range n.ownerRefs {
		gv := ownerRefGV(ownerRef)
		if parent, ok := ns[kube.NewResourceKey(gv.Group, ownerRef.Kind, n.ref.Namespace, ownerRef.Name)]; ok {
			app := parent.getAppRecursive(ns, visited)
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

func (n *node) asResourceNode() appv1.ResourceNode {
	gv, err := schema.ParseGroupVersion(n.ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	parentRefs := make([]appv1.ResourceRef, len(n.ownerRefs))
	for _, ownerRef := range n.ownerRefs {
		ownerGvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
		ownerKey := kube.NewResourceKey(ownerGvk.Group, ownerRef.Kind, n.ref.Namespace, ownerRef.Name)
		parentRefs[0] = appv1.ResourceRef{Name: ownerRef.Name, Kind: ownerKey.Kind, Namespace: n.ref.Namespace, Group: ownerKey.Group, UID: string(ownerRef.UID)}
	}
	return appv1.ResourceNode{
		ResourceRef: appv1.ResourceRef{
			UID:       string(n.ref.UID),
			Name:      n.ref.Name,
			Group:     gv.Group,
			Version:   gv.Version,
			Kind:      n.ref.Kind,
			Namespace: n.ref.Namespace,
		},
		ParentRefs:      parentRefs,
		Info:            n.info,
		ResourceVersion: n.resourceVersion,
		NetworkingInfo:  n.networkingInfo,
		Images:          n.images,
		Health:          n.health,
	}
}

func (n *node) iterateChildren(ns map[kube.ResourceKey]*node, parents map[kube.ResourceKey]bool, action func(child appv1.ResourceNode)) {
	for childKey, child := range ns {
		if n.isParentOf(ns[childKey]) {
			if parents[childKey] {
				key := n.resourceKey()
				log.Warnf("Circular dependency detected. %s is child and parent of %s", childKey.String(), key.String())
			} else {
				action(child.asResourceNode())
				child.iterateChildren(ns, newResourceKeySet(parents, n.resourceKey()), action)
			}
		}
	}
}
