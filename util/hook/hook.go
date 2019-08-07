package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/helm/hook"
	"github.com/argoproj/argo-cd/util/resource"
)

func IsHook(obj *unstructured.Unstructured) bool {
	for _, hookType := range Types(obj) {
		switch hookType {
		case v1alpha1.HookTypeSkip:
			return !Skip(obj)
		default:
			return true
		}
	}
	return false
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
	// we ignore
	if len(types) == 0 {
		for _, t := range hook.Types(obj) {
			types = append(types, t.HookType())
		}
	}
	return types
}

func DeletePolicies(obj *unstructured.Unstructured) []v1alpha1.HookDeletePolicy {
	var policies []v1alpha1.HookDeletePolicy
	for _, text := range resource.GetAnnotationCSVs(obj, common.AnnotationKeyHookDeletePolicy) {
		p, ok := v1alpha1.NewHookDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	for _, p := range hook.DeletePolicies(obj) {
		policies = append(policies, p.DeletePolicy())
	}
	return policies
}
