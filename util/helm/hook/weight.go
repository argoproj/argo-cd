package hook

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// note that we do not take into account if this is or is not a hook, caller should check
func Weight(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()["helm.sh/hook-weight"]
	if ok {
		value, err := strconv.ParseInt(text, 10, 0)
		if err == nil {
			return int(value)
		}
	}
	return 0
}
