package kubemeta

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// objectMeta is the only part of the document that gets materialized; v2 skips
// the rest while decoding.
type objectMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// KubeJson holds the identifying fields extracted from a Kubernetes object.
type KubeJson struct {
	meta  objectMeta
	gvk   schema.GroupVersionKind
	empty bool
}

var nullLiteral = []byte("null")

// NewKubeJson parses the identifying fields out of a Kubernetes object's JSON.
// It accepts and rejects exactly what a json.Unmarshal into an
// unstructured.Unstructured does.
func NewKubeJson(data []byte) (*KubeJson, error) {
	// Absent resource. Unmarshalling into an **Unstructured left the pointer
	// nil for these rather than erroring.
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, nullLiteral) {
		return &KubeJson{empty: true}, nil
	}

	k := &KubeJson{}
	// Duplicates are last-wins as before. Rejecting them would mean tracking
	// every name seen in an object even while skipping it: 178KB/1161 allocs to
	// read four fields out of a 64KB ConfigMap, versus 64B/1.
	if v2Err := jsonv2.Unmarshal(trimmed, &k.meta, jsontext.AllowDuplicateNames(true)); v2Err != nil {
		// v2 rejects a non-string identifying field where v1 read it as "".
		// SplitYAML accepts `name: 123` and it survives json.Marshal as a
		// number, so erroring would fail a whole resource tree over one object
		// that used to render.
		if err := k.unmarshalUnstructured(trimmed); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object metadata: %w (strict decode: %w)", err, v2Err)
		}
	}

	// UnstructuredJSONScheme.Decode rejects an unresolvable kind (absent, or an
	// apiVersion with too many slashes). Without this the caller gets a zero
	// ResourceKey that silently matches nothing.
	k.gvk = groupVersionKind(k.meta.APIVersion, k.meta.Kind)
	if k.gvk.Kind == "" {
		return nil, runtime.NewMissingKindErr(string(trimmed))
	}
	return k, nil
}

// unmarshalUnstructured is the v1 decode, to reproduce its lenience on input v2
// refuses. Its getters return "" for a non-string field.
func (k *KubeJson) unmarshalUnstructured(data []byte) error {
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, u); err != nil {
		return err
	}
	k.meta.APIVersion = u.GetAPIVersion()
	k.meta.Kind = u.GetKind()
	k.meta.Metadata.Name = u.GetName()
	k.meta.Metadata.Namespace = u.GetNamespace()
	return nil
}

// groupVersionKind matches unstructured.Unstructured.GroupVersionKind, which
// discards the kind when the apiVersion will not parse.
func groupVersionKind(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}
	}
	return gv.WithKind(kind)
}

// IsEmpty reports whether the document was the absent-resource sentinel.
func (k *KubeJson) IsEmpty() bool { return k.empty }

func (k *KubeJson) GetAPIVersion() string { return k.meta.APIVersion }

func (k *KubeJson) GetKind() string { return k.meta.Kind }

func (k *KubeJson) GetNamespace() string { return k.meta.Metadata.Namespace }

func (k *KubeJson) GetName() string { return k.meta.Metadata.Name }

func (k *KubeJson) GroupVersionKind() schema.GroupVersionKind { return k.gvk }

func GetResourceKey(obj *KubeJson) kube.ResourceKey {
	gvk := obj.GroupVersionKind()
	return kube.NewResourceKey(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
}
