package resource

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetRevision(obj *unstructured.Unstructured) int64 {
	if obj == nil {
		return 0
	}
	text := obj.GetAnnotations()["deployment.kubernetes.io/revision"]
	revision, _ := strconv.ParseInt(text, 10, 64)
	return revision
}
