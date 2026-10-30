package managedfields

import (
	"bytes"
	"fmt"
	"slices"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
	"sigs.k8s.io/structured-merge-diff/v6/value"
)

// Normalize will compare the live and config states. If config mutates
// a field that belongs to one of the trustedManagers it will remove
// that field from both live and config objects and return the normalized
// objects in this order. This function won't modify the live and config
// parameters. If pt is nil, the normalization will use a deduced parseable
// type which means that lists and maps are manipulated atomically.
// It is a no-op if no trustedManagers is provided. It is also a no-op if
// live or config are nil.
func Normalize(live, config *unstructured.Unstructured, trustedManagers []string, pt *typed.ParseableType) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	if len(trustedManagers) == 0 {
		return nil, nil, nil
	}
	if live == nil || config == nil {
		return nil, nil, nil
	}

	liveCopy := live.DeepCopy()
	configCopy := config.DeepCopy()
	normalized := false

	results, err := newTypedResults(liveCopy, configCopy, pt)
	// error might happen if the resources are not parsable and so cannot be normalized
	if err != nil {
		log.Debugf("error building typed results: %v", err)
		return liveCopy, configCopy, nil
	}

	for _, mf := range live.GetManagedFields() {
		if slices.Contains(trustedManagers, mf.Manager) {
			err := normalize(mf, results)
			if err != nil {
				return nil, nil, fmt.Errorf("error normalizing manager %s: %w", mf.Manager, err)
			}
			normalized = true
		}
	}

	if !normalized {
		return liveCopy, configCopy, nil
	}
	lvu := results.live.AsValue().Unstructured()
	l, ok := lvu.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("error converting live typedValue: expected map got %T", lvu)
	}
	normLive := &unstructured.Unstructured{Object: l}

	cvu := results.config.AsValue().Unstructured()
	c, ok := cvu.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("error converting config typedValue: expected map got %T", cvu)
	}
	normConfig := &unstructured.Unstructured{Object: c}
	return normLive, normConfig, nil
}

// normalize will check if the modified set has fields that are present
// in the managed fields entry. If so, it will remove the fields from
// the live and config objects so it is ignored in diffs.
func normalize(mf metav1.ManagedFieldsEntry, tr *typedResults) error {
	mfs := &fieldpath.Set{}
	err := mfs.FromJSON(bytes.NewReader(mf.FieldsV1.GetRawBytes()))
	if err != nil {
		return err
	}
	intersect := mfs.Intersection(tr.comparison.Modified)
	if intersect.Empty() {
		return nil
	}
	tr.live = tr.live.RemoveItems(intersect)
	tr.config = tr.config.RemoveItems(intersect)
	return nil
}

type typedResults struct {
	live       *typed.TypedValue
	config     *typed.TypedValue
	comparison *typed.Comparison
}

// newTypedResults will convert live and config into a TypedValue using the given pt
// and compare them. Returns a typedResults with the converted types and the comparison.
// If pt is nil, will use the DeducedParseableType.
func newTypedResults(live, config *unstructured.Unstructured, pt *typed.ParseableType) (*typedResults, error) {
	if pt == nil {
		pt = &typed.DeducedParseableType
	}
	typedLive, err := pt.FromUnstructured(live.Object)
	if err != nil {
		return nil, fmt.Errorf("error creating typedLive: %w", err)
	}

	typedConfig, err := pt.FromUnstructured(config.Object)
	if err != nil {
		return nil, fmt.Errorf("error creating typedConfig: %w", err)
	}
	comparison, err := typedLive.Compare(typedConfig)
	if err != nil {
		return nil, fmt.Errorf("error comparing typed resources: %w", err)
	}
	return &typedResults{
		live:       typedLive,
		config:     typedConfig,
		comparison: comparison,
	}, nil
}

// IgnoredPaths returns the paths that Normalize would remove from live and from
// config for the given trusted managers: the fields those managers own that
// differ between live and config. Paths are sequences of map keys (string) and
// list indexes (int) into live.Object and config.Object respectively, so a
// caller that builds an apply target can carry the live values and drop the
// config values at exactly those locations.
func IgnoredPaths(live, config *unstructured.Unstructured, trustedManagers []string, pt *typed.ParseableType) (livePaths, configPaths [][]any, err error) {
	if len(trustedManagers) == 0 || live == nil || config == nil {
		return nil, nil, nil
	}
	results, err := newTypedResults(live.DeepCopy(), config.DeepCopy(), pt)
	if err != nil {
		log.Debugf("error building typed results: %v", err)
		return nil, nil, nil
	}
	ignored := fieldpath.NewSet()
	for _, mf := range live.GetManagedFields() {
		if !slices.Contains(trustedManagers, mf.Manager) || mf.FieldsV1 == nil {
			continue
		}
		mfs := &fieldpath.Set{}
		if err := mfs.FromJSON(bytes.NewReader(mf.FieldsV1.GetRawBytes())); err != nil {
			return nil, nil, fmt.Errorf("error parsing managed fields of manager %s: %w", mf.Manager, err)
		}
		ignored = ignored.Union(mfs.Intersection(results.comparison.Modified))
	}
	if ignored.Empty() {
		return nil, nil, nil
	}
	ignored.Leaves().Iterate(func(p fieldpath.Path) {
		if jp, ok := toJSONPath(live.Object, p); ok {
			livePaths = append(livePaths, jp)
		}
		if jp, ok := toJSONPath(config.Object, p); ok {
			configPaths = append(configPaths, jp)
		}
	})
	return livePaths, configPaths, nil
}

// toJSONPath resolves a structured-merge-diff field path (which addresses list
// items by key or value) into map keys and list indexes of the given object.
func toJSONPath(root any, p fieldpath.Path) ([]any, bool) {
	cur := root
	out := make([]any, 0, len(p))
	for _, pe := range p {
		switch {
		case pe.FieldName != nil:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			cur, ok = m[*pe.FieldName]
			if !ok {
				return nil, false
			}
			out = append(out, *pe.FieldName)
		case pe.Index != nil:
			l, ok := cur.([]any)
			if !ok || *pe.Index < 0 || *pe.Index >= len(l) {
				return nil, false
			}
			cur = l[*pe.Index]
			out = append(out, *pe.Index)
		case pe.Key != nil:
			l, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			idx := -1
			for i, item := range l {
				im, ok := item.(map[string]any)
				if !ok {
					continue
				}
				match := true
				for _, f := range *pe.Key {
					if !value.Equals(value.NewValueInterface(im[f.Name]), f.Value) {
						match = false
						break
					}
				}
				if match {
					idx = i
					break
				}
			}
			if idx < 0 {
				return nil, false
			}
			cur = l[idx]
			out = append(out, idx)
		case pe.Value != nil:
			l, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			idx := -1
			for i, item := range l {
				if value.Equals(value.NewValueInterface(item), *pe.Value) {
					idx = i
					break
				}
			}
			if idx < 0 {
				return nil, false
			}
			cur = l[idx]
			out = append(out, idx)
		default:
			return nil, false
		}
	}
	return out, true
}
