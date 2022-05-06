package managedfields

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8smanagedfields "k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
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
	results, err := newTypedResults(liveCopy, configCopy, pt)
	if err != nil {
		return nil, nil, fmt.Errorf("error building typed results: %s", err)
	}

	normalized := false
	for _, mf := range live.GetManagedFields() {
		if trustedManager(mf.Manager, trustedManagers) {
			err := normalize(mf, results)
			if err != nil {
				return nil, nil, fmt.Errorf("error normalizing manager %s: %s", mf.Manager, err)
			}
			normalized = true
		}
	}

	if !normalized {
		return liveCopy, configCopy, nil
	}
	lvu := results.live.AsValue().Unstructured()
	l, ok := lvu.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("error converting live typedValue: expected map got %T", lvu)
	}
	normLive := &unstructured.Unstructured{Object: l}

	cvu := results.config.AsValue().Unstructured()
	c, ok := cvu.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("error converting config typedValue: expected map got %T", cvu)
	}
	normConfig := &unstructured.Unstructured{Object: c}
	return normLive, normConfig, nil
}

// normalize will check if the modified set has fields that are present
// in the managed fields entry. If so, it will remove the fields from
// the live and config objects so it is ignored in diffs.
func normalize(mf v1.ManagedFieldsEntry, tr *typedResults) error {
	mfs := &fieldpath.Set{}
	err := mfs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
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
// and compare them. Returns a typedResults with the coverted types and the comparison.
// If pt is nil, will use the DeducedParseableType.
func newTypedResults(live, config *unstructured.Unstructured, pt *typed.ParseableType) (*typedResults, error) {
	typedLive, err := pt.FromUnstructured(live.Object)
	if err != nil {
		return nil, fmt.Errorf("error creating typedLive: %s", err)
	}

	typedConfig, err := pt.FromUnstructured(config.Object)
	if err != nil {
		return nil, fmt.Errorf("error creating typedConfig: %s", err)
	}
	comparison, err := typedLive.Compare(typedConfig)
	if err != nil {
		return nil, fmt.Errorf("error comparing typed resources: %s", err)
	}
	return &typedResults{
		live:       typedLive,
		config:     typedConfig,
		comparison: comparison,
	}, nil
}

// trustedManager will return true if trustedManagers contains curManager.
// Returns false otherwise.
func trustedManager(curManager string, trustedManagers []string) bool {
	for _, m := range trustedManagers {
		if m == curManager {
			return true
		}
	}
	return false
}

func ResolveParseableType(gvk schema.GroupVersionKind, parser *k8smanagedfields.GvkParser) *typed.ParseableType {
	if parser == nil {
		return &typed.DeducedParseableType
	}

	gvkNameMap := getGvkMap(parser)
	name := gvkNameMap[gvk]

	p := StaticParser()
	if p == nil || name == "" {
		return parser.Type(gvk)
	}
	pt := p.Type(name)
	if pt.IsValid() {
		return &pt
	}
	return parser.Type(gvk)
}

var gvkMap map[schema.GroupVersionKind]string
var extractOnce sync.Once

func getGvkMap(parser *k8smanagedfields.GvkParser) map[schema.GroupVersionKind]string {
	extractOnce.Do(func() {
		gvkMap = extractGvkMap(parser)
	})
	return gvkMap
}

func extractGvkMap(parser *k8smanagedfields.GvkParser) map[schema.GroupVersionKind]string {
	results := make(map[schema.GroupVersionKind]string)

	value := reflect.ValueOf(parser)
	gvkValue := reflect.Indirect(value).FieldByName("gvks")
	iter := gvkValue.MapRange()
	for iter.Next() {
		group := iter.Key().FieldByName("Group").String()
		version := iter.Key().FieldByName("Version").String()
		kind := iter.Key().FieldByName("Kind").String()
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		}
		name := iter.Value().String()
		results[gvk] = name
	}
	return results
}
