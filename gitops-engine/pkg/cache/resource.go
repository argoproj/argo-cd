package cache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
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
	// Resource stores the raw manifest when compression is disabled (original behavior)
	Resource *unstructured.Unstructured
	// compressedManifest stores the compressed serialized manifest when compression is enabled.
	// Use SetManifest/GetManifest to access.
	compressedManifest []byte

	// manifestStorage records which serialization format was used
	manifestStorage ManifestStorageType
	// manifestCompression records which compression algorithm was used
	manifestCompression ManifestCompressionType

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

// iterateChildrenV2 is a depth-first traversal of the graph of resources starting from the current resource.
func (r *Resource) iterateChildrenV2(graph map[kube.ResourceKey]map[types.UID]*Resource, ns map[kube.ResourceKey]*Resource, actionCallState map[kube.ResourceKey]callState, action func(err error, child *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool) {
	key := r.ResourceKey()
	if actionCallState[key] == completed {
		return
	}
	// this indicates that we've started processing this node's children
	actionCallState[key] = inProgress
	defer func() {
		// this indicates that we've finished processing this node's children
		actionCallState[key] = completed
	}()
	children, ok := graph[key]
	if !ok || children == nil {
		return
	}
	for _, child := range children {
		childKey := child.ResourceKey()
		// For cross-namespace relationships, child might not be in ns, so use it directly from graph
		switch actionCallState[childKey] {
		case inProgress:
			// Since we encountered a node that we're currently processing, we know we have a circular dependency.
			_ = action(fmt.Errorf("circular dependency detected. %s is child and parent of %s", childKey.String(), key.String()), child, ns)
		case notCalled:
			if action(nil, child, ns) {
				child.iterateChildrenV2(graph, ns, actionCallState, action)
			}
		}
	}
}

// SetManifest compresses and stores the resource manifest using the default codec
// (JSON serialization + gzip-bestspeed compression).
// Pass nil to clear the stored manifest.
func (r *Resource) SetManifest(un *unstructured.Unstructured) error {
	if un == nil {
		r.compressedManifest = nil
		return nil
	}
	return r.SetManifestWithCodec(un, ManifestStorageJSON, ManifestCompressionGZipBestSpeed)
}

// SetManifestWithCodec serializes and compresses the resource manifest using the specified
// storage type and compression type.
func (r *Resource) SetManifestWithCodec(un *unstructured.Unstructured, storageType ManifestStorageType, compressionType ManifestCompressionType) error {
	if un == nil {
		r.compressedManifest = nil
		return nil
	}

	storageType = normalizeManifestStorageType(storageType)
	compressionType = normalizeManifestCompressionType(compressionType)

	data, err := serializeManifestObject(un.Object, storageType)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest (storage=%s): %w", storageType, err)
	}

	compressed, err := compressManifestData(data, compressionType)
	if err != nil {
		return fmt.Errorf("failed to compress manifest (compression=%s): %w", compressionType, err)
	}

	r.compressedManifest = compressed
	r.manifestStorage = storageType
	r.manifestCompression = compressionType
	return nil
}

// GetManifest returns the stored resource manifest.
// If compression is enabled, it decompresses from compressedManifest.
// If compression is disabled, it returns the raw Resource field.
// Returns nil if no manifest is stored.
func (r *Resource) GetManifest() (*unstructured.Unstructured, error) {
	if r.Resource != nil {
		return r.Resource, nil
	}
	if r.compressedManifest == nil {
		return nil, nil
	}

	data, err := decompressManifestData(r.compressedManifest, r.manifestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress manifest (compression=%s): %w", r.manifestCompression, err)
	}

	obj, err := deserializeManifestObject(data, r.manifestStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize manifest (storage=%s): %w", r.manifestStorage, err)
	}

	return &unstructured.Unstructured{Object: obj}, nil
}

// HasManifest returns true if a manifest is stored (either raw or compressed).
func (r *Resource) HasManifest() bool {
	return r.Resource != nil || r.compressedManifest != nil
}
