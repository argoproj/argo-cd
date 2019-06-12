package resource

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func HasAnnotationOption(obj *unstructured.Unstructured, key, val string) bool {
	for _, item := range strings.Split(obj.GetAnnotations()[key], ",") {
		if strings.TrimSpace(item) == val {
			return true
		}
	}
	return false
}
