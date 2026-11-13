package diff

import (
	"errors"
	"fmt"
	"slices"

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
	if manager := diffConfig.Manager(); manager != "" {
		// Trusting the manager this sync applies with would ignore every field
		// it owns, which is everything git has ever applied: no change from
		// git would ever be applied again.
		trusted := false
		for _, ignore := range ignores {
			trusted = trusted || slices.Contains(ignore.ManagedFieldsManagers, manager)
		}
		for _, override := range diffConfig.Overrides() {
			trusted = trusted || slices.Contains(override.IgnoreDifferences.ManagedFieldsManagers, manager)
		}
		if trusted {
			return nil, fmt.Errorf("managedFieldsManagers trusts %q, the manager this sync applies with: every field it owns would be ignored", manager)
		}
	}
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

// PointerPaths returns the paths the jsonPointers rules remove from one object:
// the rules that address list items by position. They resolve to the same path
// in the manifest and in the cluster, which is what makes them worth checking
// against both.
func (r *IgnoredPathsResolver) PointerPaths(obj *unstructured.Unstructured) ([]jqpath.Path, error) {
	if obj == nil {
		return nil, nil
	}
	paths, err := r.ignoreNormalizer.PointerPaths(obj)
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
