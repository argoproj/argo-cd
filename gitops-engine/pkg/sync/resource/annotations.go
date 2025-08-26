package resource

import (
	"strings"
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
	// may for de-duping
	valuesToBool := make(map[string]bool)
	for _, item := range strings.Split(obj.GetAnnotations()[key], ",") {
		val := strings.TrimSpace(item)
		if val != "" {
			valuesToBool[val] = true
		}
	}
	var values []string
	for val := range valuesToBool {
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
