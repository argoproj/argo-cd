package cache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

// Resource holds the information about Kubernetes resource, ownership references and optional information
type Resource struct {
	// ResourceVersion holds most recent observed resource version
	ResourceVersion string
	// Resource reference
	Ref corev1.ObjectReference
	// References to resource owners
	OwnerRefs []metav1.OwnerReference
	// Optional creation timestamp of the resource
	CreationTimestamp *metav1.Time
	// Optional additional information about the resource
	Info any
	// Optional whole resource manifest
	Resource *unstructured.Unstructured

	// answers if resource is inferred parent of provided resource
	isInferredParentOf func(key kube.ResourceKey) bool
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

// setOwnerRef adds or removes specified owner reference
func (r *Resource) setOwnerRef(ref metav1.OwnerReference, add bool) {
	index := -1
	for i, item := range r.OwnerRefs {
		if item.UID == ref.UID {
			index = i
			break
		}
	}
	added := index > -1
	if add != added {
		if add {
			r.OwnerRefs = append(r.OwnerRefs, ref)
		} else {
			r.OwnerRefs = append(r.OwnerRefs[:index], r.OwnerRefs[index+1:]...)
		}
	}
}

func (r *Resource) toOwnerRef() metav1.OwnerReference {
	return metav1.OwnerReference{UID: r.Ref.UID, Name: r.Ref.Name, Kind: r.Ref.Kind, APIVersion: r.Ref.APIVersion}
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

func (r *Resource) iterateChildren(ns map[kube.ResourceKey]*Resource, parents map[kube.ResourceKey]bool, action func(err error, child *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool) {
	for childKey, child := range ns {
		if r.isParentOf(ns[childKey]) {
			if parents[childKey] {
				key := r.ResourceKey()
				_ = action(fmt.Errorf("circular dependency detected. %s is child and parent of %s", childKey.String(), key.String()), child, ns)
			} else if action(nil, child, ns) {
				child.iterateChildren(ns, newResourceKeySet(parents, r.ResourceKey()), action)
			}
		}
	}
}

// iterateChildrenV2 is a depth-first traversal of the graph of resources starting from the current resource.
func (r *Resource) iterateChildrenV2(graph map[kube.ResourceKey]map[types.UID]*Resource, ns map[kube.ResourceKey]*Resource, visited map[kube.ResourceKey]int, action func(err error, child *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool) {
	key := r.ResourceKey()
	if visited[key] == 2 {
		return
	}
	// this indicates that we've started processing this node's children
	visited[key] = 1
	defer func() {
		// this indicates that we've finished processing this node's children
		visited[key] = 2
	}()
	children, ok := graph[key]
	if !ok || children == nil {
		return
	}
	for _, c := range children {
		childKey := c.ResourceKey()
		child := ns[childKey]
		if visited[childKey] == 1 {
			// Since we encountered a node that we're currently processing, we know we have a circular dependency.
			_ = action(fmt.Errorf("circular dependency detected. %s is child and parent of %s", childKey.String(), key.String()), child, ns)
		} else if visited[childKey] == 0 {
			if action(nil, child, ns) {
				child.iterateChildrenV2(graph, ns, visited, action)
			}
		}
	}
}
