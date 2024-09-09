package managedfields

import (
	"bytes"
	"fmt"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	normalized := false

	results, err := newTypedResults(liveCopy, configCopy, pt)
	// error might happen if the resources are not parsable and so cannot be normalized
	if err != nil {
		log.Debugf("error building typed results: %v", err)
		return liveCopy, configCopy, nil
	}

	for _, mf := range live.GetManagedFields() {
		if trustedManager(mf.Manager, trustedManagers) {
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
// and compare them. Returns a typedResults with the converted types and the comparison.
// If pt is nil, will use the DeducedParseableType.
func newTypedResults(live, config *unstructured.Unstructured, pt *typed.ParseableType) (*typedResults, error) {
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
