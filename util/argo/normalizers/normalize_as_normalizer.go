package normalizers

import (
	"github.com/argoproj/gitops-engine/pkg/diff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type normalizeAsNormalizer struct {
	overrides map[schema.GroupKind]string
}

func NewNormalizeAsNormalizer(overrides map[string]v1alpha1.ResourceOverride) diff.Normalizer {
	normalizedOverrides := make(map[schema.GroupKind]string)
	for key, override := range overrides {
		if override.NormalizeAs != "" {
			group, kind, err := getGroupKindForOverrideKey(key)
			if err == nil {
				normalizedOverrides[schema.GroupKind{Group: group, Kind: kind}] = override.NormalizeAs
			}
		}
	}
	return &normalizeAsNormalizer{overrides: normalizedOverrides}
}

func (n *normalizeAsNormalizer) Normalize(un *unstructured.Unstructured) error {
	if n.overrides == nil {
		return nil
	}
	if normalizeAs, ok := n.overrides[un.GroupVersionKind().GroupKind()]; ok {
		annotations := un.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[common.AnnotationKeyNormalizeAs] = normalizeAs
		un.SetAnnotations(annotations)
	}
	return nil
}
