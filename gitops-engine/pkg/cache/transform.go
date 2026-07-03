package cache

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// cachedResource is the type stored in the informer's store by the informer
// engine (see issue #19199). It pairs the domain Resource (what the rest of
// the cluster cache wants) with just enough Kubernetes metadata that
// meta.Accessor / MetaNamespaceKeyFunc can key it for the informer's indexer.
//
// The full unstructured object is kept on Resource.Resource only when
// OnPopulateResourceInfoHandler returns cacheManifest=true — the same
// contract as the legacy path through newResource. Everything else drops
// to the floor after we've extracted the metadata needed for hierarchy
// traversal and the user-supplied Info.
type cachedResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	// Resource carries the domain payload (Ref, OwnerRefs, Info, etc.).
	// Named rather than embedded to avoid promoted-field collisions with
	// ObjectMeta (both have e.g. a notion of "Name").
	Resource *Resource
}

// DeepCopyObject satisfies runtime.Object. The returned shell holds copies
// of TypeMeta and ObjectMeta but SHARES the Resource pointer with the
// receiver — by design. The same *Resource lives in the informer's store
// and in c.resources (the shadow map), and onInformerChange mutates
// Resource.OwnerRefs under c.lock during inferred parent-ref propagation
// (updateIndexes). Deep-copying Resource here would not stop that mutation
// pattern, only hide it from readers. Callers that need an isolated
// snapshot — including the client-go cache mutation detector — must copy
// OwnerRefs themselves; the detector is therefore incompatible with this
// mode and should not be enabled together with ModeInformer.
func (cr *cachedResource) DeepCopyObject() runtime.Object {
	if cr == nil {
		return nil
	}
	return &cachedResource{
		TypeMeta:   cr.TypeMeta,
		ObjectMeta: *cr.ObjectMeta.DeepCopy(),
		Resource:   cr.Resource,
	}
}

// GetObjectKind is part of runtime.Object. It returns the embedded TypeMeta.
func (cr *cachedResource) GetObjectKind() schema.ObjectKind {
	return &cr.TypeMeta
}
