package hook

import (
	"github.com/argoproj/argo-cd/engine/common"
	"github.com/argoproj/argo-cd/engine/hook/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/resource"
)

func IsHook(obj *unstructured.Unstructured) bool {
	_, ok := obj.GetAnnotations()[common.AnnotationKeyHook]
	if ok {
		return !Skip(obj)
	}
	return helm.IsHook(obj)
}

func Skip(obj *unstructured.Unstructured) bool {
	for _, hookType := range Types(obj) {
		if hookType == v1alpha1.HookTypeSkip {
			return len(Types(obj)) == 1
		}
	}
	return false
}

func Types(obj *unstructured.Unstructured) []v1alpha1.HookType {
	var types []v1alpha1.HookType
	for _, text := range resource.GetAnnotationCSVs(obj, common.AnnotationKeyHook) {
		t, ok := v1alpha1.NewHookType(text)
		if ok {
			types = append(types, t)
		}
	}
	// we ignore Helm hooks if we have Argo hook
	if len(types) == 0 {
		for _, t := range helm.Types(obj) {
			types = append(types, t.HookType())
		}
	}
	return types
}
