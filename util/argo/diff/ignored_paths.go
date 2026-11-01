package diff

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/scheme"

	"github.com/argoproj/argo-cd/v3/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/jqpath"
)

// IgnoredPathsResolver resolves the field paths that the ignoreDifferences
// configuration removes from live and from target before they are compared:
// jsonPointers and jqPathExpressions (from the Application, from system
// resource overrides and from the ignore-differences annotation on the
// targets) and the fields owned by managedFieldsManagers.
//
// Only the ignore rules contribute. The known-types and resource-tracking
// normalizers also change the objects before a diff, but what they change is
// not ignored: it is compared in a canonical form.
//
// The ignore rules are compiled once per resolver, so build one per set of
// targets and call IgnoredPaths for each live/target pair.
type IgnoredPathsResolver struct {
	diffConfig       DiffConfig
	ignoreNormalizer normalizers.IgnoreNormalizer
	ignoreConfig     *IgnoreDiffConfig
}

// NewIgnoredPathsResolver builds a resolver for the given targets. The targets
// are needed up front because their ignore-differences annotations contribute
// rules, exactly as in Normalize.
func NewIgnoredPathsResolver(targets []*unstructured.Unstructured, diffConfig DiffConfig) (*IgnoredPathsResolver, error) {
	if diffConfig == nil {
		return nil, errors.New("invalid argument: diffConfig is nil")
	}
	ignores := resolvedIgnores(targets, diffConfig.Ignores())
	ignoreNormalizer, err := normalizers.NewIgnoreNormalizer(ignores, diffConfig.Overrides(), diffConfig.IgnoreNormalizerOpts())
	if err != nil {
		return nil, fmt.Errorf("failed to build ignore normalizer: %w", err)
	}
	return &IgnoredPathsResolver{
		diffConfig:       diffConfig,
		ignoreNormalizer: ignoreNormalizer,
		ignoreConfig:     NewIgnoreDiffConfig(diffConfig.Ignores(), diffConfig.Overrides()),
	}, nil
}

// RulePaths returns the paths the jsonPointers and jqPathExpressions rules
// remove from one object on its own. managedFieldsManagers rules need the live
// object and the manifest together and are left out; IgnoredPaths has them.
func (r *IgnoredPathsResolver) RulePaths(obj *unstructured.Unstructured) ([]jqpath.Path, error) {
	if obj == nil {
		return nil, nil
	}
	paths, err := r.ignoreNormalizer.IgnoredPaths(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ignored paths: %w", err)
	}
	return paths, nil
}

// IgnoredPaths returns the paths the ignore rules remove from live and from
// target. Paths are sequences of map keys (string) and list indexes (int) into
// live.Object and target.Object as given. It returns no paths if either object
// is nil.
func (r *IgnoredPathsResolver) IgnoredPaths(live, target *unstructured.Unstructured) (livePaths, targetPaths []jqpath.Path, err error) {
	if live == nil || target == nil {
		return nil, nil, nil
	}
	livePaths, err = r.ignoreNormalizer.IgnoredPaths(live)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve ignored paths of live resource: %w", err)
	}
	targetPaths, err = r.ignoreNormalizer.IgnoredPaths(target)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve ignored paths of target resource: %w", err)
	}

	gvk := target.GroupVersionKind()
	if ok, ignoreDiff := r.ignoreConfig.HasIgnoreDifference(gvk.Group, gvk.Kind, target.GetName(), target.GetNamespace()); ok && len(ignoreDiff.ManagedFieldsManagers) > 0 {
		pt := scheme.ResolveParseableType(gvk, r.diffConfig.GVKParser())
		mfLive, mfTarget, err := managedfields.IgnoredPaths(live, target, ignoreDiff.ManagedFieldsManagers, pt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve managed fields ignored paths: %w", err)
		}
		livePaths = append(livePaths, mfLive...)
		targetPaths = append(targetPaths, mfTarget...)
	}
	return livePaths, targetPaths, nil
}
