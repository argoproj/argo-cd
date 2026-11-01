package managedfields

import (
	"bytes"
	"fmt"
	"slices"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	smdschema "sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
	"sigs.k8s.io/structured-merge-diff/v6/value"

	"github.com/argoproj/argo-cd/v3/util/jqpath"
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
func IgnoredPaths(live, config *unstructured.Unstructured, trustedManagers []string, pt *typed.ParseableType) (livePaths, configPaths []jqpath.Path, err error) {
	if len(trustedManagers) == 0 || live == nil || config == nil {
		return nil, nil, nil
	}
	if pt == nil {
		// The cluster schema has no type for this kind (a CRD it does not
		// publish); deduce one, as Normalize does.
		pt = &typed.DeducedParseableType
	}
	// Nothing below changes either object, so no copies are needed.
	results, err := newTypedResults(live, config, pt)
	if err != nil {
		// The objects cannot be typed: duplicate keys in a list an old
		// client-side apply left behind, or a field newer than the schema.
		// That is the environment, not the rule, and the user cannot change
		// the rule to make it succeed, so the rule contributes nothing for
		// this resource, as it does at diff time (Normalize), and the sync
		// goes on. The warning is what the Debug line at diff time is not.
		log.WithFields(log.Fields{
			"kind":      live.GetKind(),
			"name":      live.GetName(),
			"namespace": live.GetNamespace(),
			"managers":  trustedManagers,
		}).Warnf("managedFieldsManagers rule skipped, the objects could not be typed: %v", err)
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
		if jp, ok := toJSONPath(live.Object, p, pt.Schema, pt.TypeRef); ok {
			livePaths = append(livePaths, jp)
		}
		if jp, ok := toJSONPath(config.Object, p, pt.Schema, pt.TypeRef); ok {
			configPaths = append(configPaths, jp)
		}
	})
	return livePaths, configPaths, nil
}

// FindKeyedItem returns the index of the item in list whose key fields equal
// want, or -1. A key field an item omits takes its value from defaults, which
// is how the API server identifies the item (a Service port without protocol
// is the TCP port). Values compare as structured-merge-diff compares them, so
// a number matches whatever Go type it was decoded as: a schema default is an
// int, a document value an int64 or float64.
func FindKeyedItem(list []any, want, defaults map[string]any) int {
	for i, item := range list {
		im, ok := item.(map[string]any)
		if !ok {
			continue
		}
		match := true
		for k, w := range want {
			v, present := im[k]
			if !present {
				v = defaults[k]
			}
			if !value.Equals(value.NewValueInterface(v), value.NewValueInterface(w)) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// toJSONPath resolves a structured-merge-diff field path (which addresses list
// items by key or value) into map keys and list indexes of the given object.
func toJSONPath(root any, p fieldpath.Path, s *smdschema.Schema, ref smdschema.TypeRef) ([]any, bool) {
	cur := root
	out := make([]any, 0, len(p))
	for _, pe := range p {
		atom, hasAtom := resolveAtom(s, ref)
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
			ref = smdschema.TypeRef{}
			if hasAtom && atom.Map != nil {
				if sf, ok := atom.FindField(*pe.FieldName); ok {
					ref = sf.Type
				} else {
					ref = atom.Map.ElementType
				}
			}
		case pe.Index != nil:
			l, ok := cur.([]any)
			if !ok || *pe.Index < 0 || *pe.Index >= len(l) {
				return nil, false
			}
			cur = l[*pe.Index]
			out = append(out, *pe.Index)
			ref = listElementRef(atom, hasAtom)
		case pe.Key != nil:
			l, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			elemRef := listElementRef(atom, hasAtom)
			elemAtom, hasElem := resolveAtom(s, elemRef)
			want, defaults := map[string]any{}, map[string]any{}
			for _, f := range *pe.Key {
				want[f.Name] = f.Value.Unstructured()
				if hasElem && elemAtom.Map != nil {
					if sf, ok := elemAtom.FindField(f.Name); ok && sf.Default != nil {
						defaults[f.Name] = sf.Default
					}
				}
			}
			idx := FindKeyedItem(l, want, defaults)
			if idx < 0 {
				return nil, false
			}
			cur = l[idx]
			out = append(out, idx)
			ref = elemRef
		case pe.Value != nil:
			l, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			idx := slices.IndexFunc(l, func(item any) bool { return value.Equals(value.NewValueInterface(item), *pe.Value) })
			if idx < 0 {
				return nil, false
			}
			cur = l[idx]
			out = append(out, idx)
			ref = listElementRef(atom, hasAtom)
		default:
			return nil, false
		}
	}
	return out, true
}

func resolveAtom(s *smdschema.Schema, ref smdschema.TypeRef) (smdschema.Atom, bool) {
	if s == nil {
		return smdschema.Atom{}, false
	}
	return s.Resolve(ref)
}

func listElementRef(atom smdschema.Atom, hasAtom bool) smdschema.TypeRef {
	if !hasAtom || atom.List == nil {
		return smdschema.TypeRef{}
	}
	return atom.List.ElementType
}
