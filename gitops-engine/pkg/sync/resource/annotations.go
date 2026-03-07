package resource

import (
	"strings"

	"k8s.io/utils/ptr"
)

// AnnotationGetter defines the operations required to inspect if a resource
// has annotations
type AnnotationGetter interface {
	GetAnnotations() map[string]string
}

// GetAnnotationCSVs will return the value of the annotation identified by
// the given key. If the annotation has comma separated values, the returned
// list will contain all deduped values.
func GetAnnotationCSVs(obj AnnotationGetter, key string) []string {
	// map for de-duping
	seen := make(map[string]bool)
	var values []string
	for _, item := range strings.Split(obj.GetAnnotations()[key], ",") {
		val := strings.TrimSpace(item)
		if val == "" {
			continue
		}
		if seen[val] {
			continue
		}
		seen[val] = true
		values = append(values, val)
	}
	return values
}

// HasAnnotationOption will return if the given obj has an annotation defined
// as the given key and has in its values, the occurrence of val.
func HasAnnotationOption(obj AnnotationGetter, key, val string) bool {
	for _, item := range GetAnnotationCSVs(obj, key) {
		if item == val {
			return true
		}
	}
	return false
}

// GetAnnotationOptionValue will return the value of an option inside the
// annotation defined as the given key.
// This function only support options that are defined as key=value and not standalone.
func GetAnnotationOptionValue(obj AnnotationGetter, annotation, optionKey string) *string {
	prefix := optionKey + "="
	for _, item := range GetAnnotationCSVs(obj, annotation) {
		if val, found := strings.CutPrefix(item, prefix); found {
			return ptr.To(val)
		}
	}
	return nil
}
