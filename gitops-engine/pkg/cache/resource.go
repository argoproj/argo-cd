package cache

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/s2"
	"github.com/vmihailenco/msgpack/v5"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// ManifestCompressionType defines the compression algorithm for cached manifests
type ManifestCompressionType string

// ManifestStorageType defines the serialization format for cached manifests
type ManifestStorageType string

const (
	ManifestCompressionGZip           ManifestCompressionType = "gzip"
	ManifestCompressionGZipDefault    ManifestCompressionType = "gzip-default"
	ManifestCompressionGZipBestSpeed  ManifestCompressionType = "gzip-bestspeed"
	ManifestCompressionS2Encode       ManifestCompressionType = "s2-encode"
	ManifestCompressionS2EncodeBetter ManifestCompressionType = "s2-encodebetter"
	ManifestCompressionZLib           ManifestCompressionType = "zlib"
	ManifestCompressionNone           ManifestCompressionType = "none"

	ManifestStorageJSON     ManifestStorageType = "json"
	ManifestStorageJSONIter ManifestStorageType = "jsoniter"
	ManifestStorageMsgPack  ManifestStorageType = "msgpack"
)

// normalizeManifestCompressionType normalizes a compression type string to a known value.
// "gzip" is normalized to "gzip-bestspeed"; unknown values default to "gzip-bestspeed".
func normalizeManifestCompressionType(ct ManifestCompressionType) ManifestCompressionType {
	switch ct {
	case ManifestCompressionGZip:
		return ManifestCompressionGZipBestSpeed
	case ManifestCompressionGZipDefault, ManifestCompressionGZipBestSpeed,
		ManifestCompressionS2Encode, ManifestCompressionS2EncodeBetter,
		ManifestCompressionZLib, ManifestCompressionNone:
		return ct
	default:
		return ManifestCompressionGZipBestSpeed
	}
}

// normalizeManifestStorageType normalizes a storage type string to a known value.
// Unknown values default to "json".
func normalizeManifestStorageType(st ManifestStorageType) ManifestStorageType {
	switch st {
	case ManifestStorageJSON, ManifestStorageJSONIter, ManifestStorageMsgPack:
		return st
	default:
		return ManifestStorageJSON
	}
}

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

// serializeManifestObject serializes a map to bytes using the specified storage type.
func serializeManifestObject(obj map[string]any, storageType ManifestStorageType) ([]byte, error) {
	switch storageType {
	case ManifestStorageJSONIter:
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("jsoniter marshal failed: %w", err)
		}
		return data, nil
	case ManifestStorageMsgPack:
		data, err := msgpack.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("msgpack marshal failed: %w", err)
		}
		return data, nil
	default: // ManifestStorageJSON
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("json marshal failed: %w", err)
		}
		return data, nil
	}
}

// deserializeManifestObject deserializes bytes to a map using the specified storage type.
func deserializeManifestObject(data []byte, storageType ManifestStorageType) (map[string]any, error) {
	switch storageType {
	case ManifestStorageJSONIter:
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("jsoniter unmarshal failed: %w", err)
		}
		return obj, nil
	case ManifestStorageMsgPack:
		var obj map[string]any
		if err := msgpack.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("msgpack unmarshal failed: %w", err)
		}
		normalizeManifestValue(obj)
		return obj, nil
	default: // ManifestStorageJSON
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}
		return obj, nil
	}
}

// compressManifestData compresses bytes using the specified compression type.
func compressManifestData(data []byte, compressionType ManifestCompressionType) ([]byte, error) {
	switch compressionType {
	case ManifestCompressionGZipDefault:
		var buf bytes.Buffer
		gz, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		if _, err := gz.Write(data); err != nil {
			return nil, fmt.Errorf("failed to write gzip data: %w", err)
		}
		if err := gz.Close(); err != nil {
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}
		return buf.Bytes(), nil
	case ManifestCompressionGZipBestSpeed:
		var buf bytes.Buffer
		gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		if _, err := gz.Write(data); err != nil {
			return nil, fmt.Errorf("failed to write gzip data: %w", err)
		}
		if err := gz.Close(); err != nil {
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}
		return buf.Bytes(), nil
	case ManifestCompressionS2Encode:
		return s2.Encode(nil, data), nil
	case ManifestCompressionS2EncodeBetter:
		return s2.EncodeBetter(nil, data), nil
	case ManifestCompressionZLib:
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			return nil, fmt.Errorf("failed to write zlib data: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("failed to close zlib writer: %w", err)
		}
		return buf.Bytes(), nil
	case ManifestCompressionNone:
		return data, nil
	default:
		return nil, fmt.Errorf("unknown compression type: %s", compressionType)
	}
}

// decompressManifestData decompresses bytes using the specified compression type.
func decompressManifestData(data []byte, compressionType ManifestCompressionType) ([]byte, error) {
	switch compressionType {
	case ManifestCompressionGZipDefault, ManifestCompressionGZipBestSpeed:
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gz.Close()
		result, err := io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("failed to read gzip data: %w", err)
		}
		return result, nil
	case ManifestCompressionS2Encode, ManifestCompressionS2EncodeBetter:
		result, err := s2.Decode(nil, data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode s2 data: %w", err)
		}
		return result, nil
	case ManifestCompressionZLib:
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create zlib reader: %w", err)
		}
		defer r.Close()
		result, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read zlib data: %w", err)
		}
		return result, nil
	case ManifestCompressionNone:
		return data, nil
	default:
		return nil, fmt.Errorf("unknown compression type: %s", compressionType)
	}
}

// normalizeManifestValue normalizes msgpack-decoded values to be compatible with
// unstructured.Unstructured expectations. Specifically
// Converts integer types (uint64, int64, uint32, int32, int, uint) to float64
// and Converts map[any]any to map[string]any
func normalizeManifestValue(obj map[string]any) {
	for k, v := range obj {
		obj[k] = normalizeValue(v)
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		normalizeManifestValue(val)
		return val
	case map[any]any:
		converted := make(map[string]any, len(val))
		for mk, mv := range val {
			converted[fmt.Sprintf("%v", mk)] = normalizeValue(mv)
		}
		return converted
	case []any:
		for i, item := range val {
			val[i] = normalizeValue(item)
		}
		return val
	case uint64:
		return float64(val)
	case int64:
		return float64(val)
	case uint32:
		return float64(val)
	case int32:
		return float64(val)
	case int:
		return float64(val)
	case uint:
		return float64(val)
	default:
		return v
	}
}
