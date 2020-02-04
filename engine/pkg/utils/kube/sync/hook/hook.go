package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	helmhook "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/hook/helm"
	resourceutil "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/resource"
)

func IsHook(obj *unstructured.Unstructured) bool {
	_, ok := obj.GetAnnotations()[common.AnnotationKeyHook]
	if ok {
		return !Skip(obj)
	}
	return helmhook.IsHook(obj)
}

func Skip(obj *unstructured.Unstructured) bool {
	for _, hookType := range Types(obj) {
		if hookType == common.HookTypeSkip {
			return len(Types(obj)) == 1
		}
	}
	return false
}

func Types(obj *unstructured.Unstructured) []common.HookType {
	var types []common.HookType
	for _, text := range resourceutil.GetAnnotationCSVs(obj, common.AnnotationKeyHook) {
		t, ok := common.NewHookType(text)
		if ok {
			types = append(types, t)
		}
	}
	// we ignore Helm hooks if we have Argo hook
	if len(types) == 0 {
		for _, t := range helmhook.Types(obj) {
			types = append(types, t.HookType())
		}
	}
	return types
}
