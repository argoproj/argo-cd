package kubemeta

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// objectMeta declares only the fields we care about. encoding/json/v2 discards
// every other member of the document while decoding, without materializing it.
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
	empty bool
}

var (
	nullLiteral = []byte("null")
	emptyObject = []byte("{}")
	emptyArray  = []byte("[]")
)

// NewKubeJson parses just the identifying fields out of a Kubernetes object's
// JSON. It returns an error for malformed JSON.
func NewKubeJson(data []byte) (*KubeJson, error) {
	// IsEmpty mirrors "the object has no members": in practice a resource's
	// state is "", whitespace, or "null" when it is absent; {} and [] are
	// handled defensively (an empty array would also fail to decode).
	trimmed := bytes.TrimSpace(data)
	switch {
	case len(trimmed) == 0,
		bytes.Equal(trimmed, nullLiteral),
		bytes.Equal(trimmed, emptyObject),
		bytes.Equal(trimmed, emptyArray):
		return &KubeJson{empty: true}, nil
	}

	k := &KubeJson{}
	// Duplicate names are allowed for two reasons. Behaviour: this replaces a
	// json.Unmarshal into unstructured, which is last-wins; erroring instead
	// would be a divergence, and every input here is json.Marshal output from a
	// Go map, which cannot produce duplicates anyway. Cost: rejecting them means
	// tracking every name seen in an object even while skipping it — 178KB and
	// 1161 allocs to read four fields out of a 64KB ConfigMap, versus 64B and 1.
	if err := jsonv2.Unmarshal(trimmed, &k.meta, jsontext.AllowDuplicateNames(true)); err != nil {
		return nil, err
	}
	return k, nil
}

// IsEmpty reports whether the document carried no members (absent state).
func (k *KubeJson) IsEmpty() bool { return k.empty }

// GetAPIVersion returns the apiVersion. v2 has already unescaped it into a Go
// string, so no manual unquoting is required.
func (k *KubeJson) GetAPIVersion() string { return k.meta.APIVersion }

func (k *KubeJson) GetKind() string { return k.meta.Kind }

func (k *KubeJson) GetNamespace() string { return k.meta.Metadata.Namespace }

func (k *KubeJson) GetName() string { return k.meta.Metadata.Name }

func (k *KubeJson) GroupVersionKind() schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(k.GetAPIVersion())
	if err != nil {
		return schema.GroupVersionKind{}
	}
	return gv.WithKind(k.GetKind())
}

// GetResourceKey builds a ResourceKey from the parsed fields.
func GetResourceKey(obj *KubeJson) kube.ResourceKey {
	gvk := obj.GroupVersionKind()
	return kube.NewResourceKey(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
}
