package cache

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// cachedResource is the type stored in the informer's store by the planned
// informer-based cluster cache (see issue #19199). It pairs the domain
// Resource (what the rest of the cluster cache wants) with just enough
// Kubernetes metadata that meta.Accessor / MetaNamespaceKeyFunc can key
// it for the informer's indexer.
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

// transformForInformer is the TransformFunc installed on a
// SharedIndexInformer by the planned informer-based cluster cache. It
// converts incoming unstructured objects to cachedResource at intake,
// collapsing the informer's store and the legacy c.resources map into
// a single source of truth.
//
// OnPopulateResourceInfoHandler is invoked here rather than in the event
// handler. Argo-cd's handler is pure (reads cluster-cache-independent
// state only) so the earlier invocation is safe; it still decides whether
// the full manifest is retained on Resource.Resource.
//
// Non-unstructured inputs (e.g. *metav1.Status from watch stream errors,
// DeletedFinalStateUnknown tombstones) pass through unchanged so callers
// upstream of this function can handle them.
func (c *clusterCache) transformForInformer(obj any) (any, error) {
	un, ok := obj.(*unstructured.Unstructured)
	if !ok || un == nil {
		return obj, nil
	}
	res := c.newResource(un)
	// handleCRDEvent -> crdVersionsToAPIResources needs spec.versions to
	// drive startMissingWatches / deleteAPIResource. Always retain the
	// full CRD manifest regardless of the populate handler's cacheManifest
	// return so direct gitops-engine users (and a future argo-cd handler
	// change) keep correct dynamic API updates.
	if res.Resource == nil && kube.IsCRD(un) {
		res.Resource = un
	}
	return &cachedResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: un.GetAPIVersion(),
			Kind:       un.GetKind(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            un.GetName(),
			Namespace:       un.GetNamespace(),
			UID:             un.GetUID(),
			ResourceVersion: un.GetResourceVersion(),
		},
		Resource: res,
	}, nil
}
