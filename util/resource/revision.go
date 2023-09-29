package resource

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetRevision(obj *unstructured.Unstructured) int64 {
	if obj == nil {
		return 0
	}
	for _, name := range []string{"deployment.kubernetes.io/revision", "rollout.argoproj.io/revision"} {
		text, ok := obj.GetAnnotations()[name]
		if ok {
			revision, _ := strconv.ParseInt(text, 10, 64)
			return revision
		}
	}

	text, ok := obj.UnstructuredContent()["revision"].(int64)
	if ok {
		return text
	}

	return 0
}
