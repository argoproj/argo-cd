package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func IsHook(obj *unstructured.Unstructured) bool {
	for _, hookType := range types(obj) {
		switch v1alpha1.HookType(hookType) {
		case v1alpha1.HookTypeSkip:
			return !Skip(obj)
		default:
			return true
		}
	}
	return false
}

func Skip(obj *unstructured.Unstructured) bool {
	for _, hookType := range types(obj) {
		if v1alpha1.HookType(hookType) == v1alpha1.HookTypeSkip {
			return len(types(obj)) == 1
		}
	}
	return false
}

func Types(obj *unstructured.Unstructured) []v1alpha1.HookType {
	var hookTypes []v1alpha1.HookType
	for _, hookType := range types(obj) {
		switch v1alpha1.HookType(hookType) {
		case v1alpha1.HookTypePreSync, v1alpha1.HookTypeSync, v1alpha1.HookTypePostSync, v1alpha1.HookTypeSyncFail:
			hookTypes = append(hookTypes, v1alpha1.HookType(hookType))
		}
	}
	return hookTypes
}

// returns a normalize list of strings
func types(obj *unstructured.Unstructured) []string {
	var hookTypes []string
	for _, hookType := range strings.Split(obj.GetAnnotations()[common.AnnotationKeyHook], ",") {
		trimmed := strings.TrimSpace(hookType)
		if len(trimmed) > 0 {
			hookTypes = append(hookTypes, trimmed)
		}
	}
	return hookTypes
}
