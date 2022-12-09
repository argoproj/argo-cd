package resource

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetAnnotationCSVs(obj *unstructured.Unstructured, key string) []string {
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

func HasAnnotationOption(obj *unstructured.Unstructured, key, val string) bool {
	for _, item := range GetAnnotationCSVs(obj, key) {
		if item == val {
			return true
		}
	}
	return false
}
