package helm

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func IsHook(obj *unstructured.Unstructured) bool {
	_, ok := obj.GetAnnotations()["helm.sh/hook"]
	return ok
}
