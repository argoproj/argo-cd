package cache

import (
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
)

type Resource struct {
	ResourceVersion string
	Ref             v1.ObjectReference
	OwnerRefs       []metav1.OwnerReference
	Info            interface{}

	// available only for root application nodes
	Resource *unstructured.Unstructured
}

func (r *Resource) ResourceKey() kube.ResourceKey {
	return kube.NewResourceKey(r.Ref.GroupVersionKind().Group, r.Ref.Kind, r.Ref.Namespace, r.Ref.Name)
}

func (r *Resource) isParentOf(child *Resource) bool {
	for i, ownerRef := range child.OwnerRefs {

		// backfill UID of inferred owner child references
		if ownerRef.UID == "" && r.Ref.Kind == ownerRef.Kind && r.Ref.APIVersion == ownerRef.APIVersion && r.Ref.Name == ownerRef.Name {
			ownerRef.UID = r.Ref.UID
			child.OwnerRefs[i] = ownerRef
			return true
		}

		if r.Ref.UID == ownerRef.UID {
			return true
		}
	}

	return false
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

func (r *Resource) iterateChildren(ns map[kube.ResourceKey]*Resource, parents map[kube.ResourceKey]bool, action func(child *Resource, namespaceResources map[kube.ResourceKey]*Resource)) {
	for childKey, child := range ns {
		if r.isParentOf(ns[childKey]) {
			if parents[childKey] {
				key := r.ResourceKey()
				log.Warnf("Circular dependency detected. %s is child and parent of %s", childKey.String(), key.String())
			} else {
				action(child, ns)
				child.iterateChildren(ns, newResourceKeySet(parents, r.ResourceKey()), action)
			}
		}
	}
}
