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
	typeToBool := make(map[v1alpha1.HookType]bool)
	for _, text := range resource.GetAnnotationCSVs(obj, common.AnnotationKeyHook) {
		hookType, ok := v1alpha1.NewHookType(text)
		if ok {
			typeToBool[hookType] = true
		}
	}
	for _, t := range hook.Types(obj) {
		typeToBool[t.HookType()] = true
	}
	// this is very complex looking, but all it really does is ensure we can't get the hook twice, as this
	// would ultimately result in running the hook twice
	var types []v1alpha1.HookType
	for t := range typeToBool {
		types = append(types, t)
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
