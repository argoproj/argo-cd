package argo

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo/normalizers"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewDiffNormalizer creates normalizer that uses Argo CD and application settings to normalize the resource prior to diffing.
func NewDiffNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride) (diff.Normalizer, error) {
	ignoreNormalizer, err := normalizers.NewIgnoreNormalizer(ignore, overrides)
	if err != nil {
		return nil, err
	}
	knownTypesNorm, err := normalizers.NewKnownTypesNormalizer(overrides)
	if err != nil {
		return nil, err
	}

	return &composableNormalizer{normalizers: []diff.Normalizer{ignoreNormalizer, knownTypesNorm}}, nil
}

type composableNormalizer struct {
	normalizers []diff.Normalizer
}

// Normalize performs resource normalization.
func (n *composableNormalizer) Normalize(un *unstructured.Unstructured) error {
	for i := range n.normalizers {
		if err := n.normalizers[i].Normalize(un); err != nil {
			return err
		}
	}
	return nil
}
