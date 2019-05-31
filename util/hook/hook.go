package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func IsHook(obj *unstructured.Unstructured) bool {
	for _, hookType := range types(obj) {
		switch HookType(hookType) {
		case HookTypeSkip:
			return !Skip(obj)
		default:
			return true
		}
	}
	return false
}

func Skip(obj *unstructured.Unstructured) bool {
	for _, hookType := range types(obj) {
		if HookType(hookType) == HookTypeSkip {
			return len(types(obj)) == 1
		}
	}
	return false
}

func Types(obj *unstructured.Unstructured) []HookType {
	var hookTypes []HookType
	for _, hookType := range types(obj) {
		switch HookType(hookType) {
		case HookTypePreSync, HookTypeSync, HookTypePostSync:
			hookTypes = append(hookTypes, HookType(hookType))
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
