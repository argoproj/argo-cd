package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/resource"
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
	for _, text := range types(obj) {
		hookType := v1alpha1.HookType(text)
		switch hookType {
		case v1alpha1.HookTypePreSync, v1alpha1.HookTypeSync, v1alpha1.HookTypePostSync, v1alpha1.HookTypeSyncFail:
			hookTypes = append(hookTypes, hookType)
		}
	}
	return hookTypes
}

func types(obj *unstructured.Unstructured) []string {
	return resource.GetAnnotationCSVs(obj, common.AnnotationKeyHook)
}

func DeletePolicies(hook *unstructured.Unstructured) []v1alpha1.HookDeletePolicy {
	var policies []v1alpha1.HookDeletePolicy
	for _, text := range resource.GetAnnotationCSVs(hook, common.AnnotationKeyHookDeletePolicy) {
		policy := v1alpha1.HookDeletePolicy(text)
		switch policy {
		case v1alpha1.HookDeletePolicyBeforeHookCreation, v1alpha1.HookDeletePolicyHookSucceeded, v1alpha1.HookDeletePolicyHookFailed:
			policies = append(policies, policy)
		}
	}
	return policies
}
