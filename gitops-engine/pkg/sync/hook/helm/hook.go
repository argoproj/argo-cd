package helm

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func IsHook(obj *unstructured.Unstructured) bool {
	value, ok := obj.GetAnnotations()["helm.sh/hook"]
	// Helm use the same annotation to identify CRD as hooks, but they are not.
	return ok && value != "crd-install"
}
