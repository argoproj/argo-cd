package diff

import (
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Normalize applies the full normalization on the lives and configs resources based
// on the provided DiffConfig.
func Normalize(lives, configs []*unstructured.Unstructured, diffConfig DiffConfig) (*NormalizationResult, error) {
	result, err := preDiffNormalize(lives, configs, diffConfig)
	if err != nil {
		return nil, err
	}
	diffNormalizer, err := newDiffNormalizer(diffConfig.Ignores(), diffConfig.Overrides(), diffConfig.IgnoreNormalizerOpts())
	if err != nil {
		return nil, err
	}

	for _, live := range result.Lives {
		if live != nil {
			err = diffNormalizer.Normalize(live)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, target := range result.Targets {
		if target != nil {
			err = diffNormalizer.Normalize(target)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// newDiffNormalizer creates normalizer that uses Argo CD and application settings to normalize the resource prior to diffing.
func newDiffNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride, opts normalizers.IgnoreNormalizerOpts) (diff.Normalizer, error) {
	ignoreNormalizer, err := normalizers.NewIgnoreNormalizer(ignore, overrides, opts)
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
