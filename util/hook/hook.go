package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func IsHook(obj *unstructured.Unstructured) bool {
	for _, hookType := range hookTypes(obj) {
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
	types := hookTypes(obj)
	for _, hookType := range types {
		if HookType(hookType) == HookTypeSkip {
			return len(types) == 1
		}
	}
	return false
}

func HookTypes(obj *unstructured.Unstructured) []HookType {
	var types []HookType
	for _, hookType := range hookTypes(obj) {
		switch HookType(hookType) {
		case HookTypePreSync, HookTypeSync, HookTypePostSync:
			types = append(types, HookType(hookType))
		}
	}
	return types
}

// returns a normalize list of strings
func hookTypes(obj *unstructured.Unstructured) []string {
	var hookTypes []string
	for _, hookType := range strings.Split(obj.GetAnnotations()[common.AnnotationKeyHook], ",") {
		trimmed := strings.TrimSpace(hookType)
		if len(trimmed) > 0 {
			hookTypes = append(hookTypes, trimmed)
		}
	}
	return hookTypes
}
