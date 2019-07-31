package helm

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/resource"
)

type HookType string

func (t HookType) Order() int {
	return map[HookType]int{
		"crd-install":  -3,
		"pre-install":  -2,
		"pre-upgrade":  -1,
		"post-upgrade": 1,
		"post-install": 2,
	}[t]
}

func (t HookType) HookType() (v1alpha1.HookType, bool) {
	hookType, ok := map[HookType]v1alpha1.HookType{
		"crd-install":  v1alpha1.HookTypePreSync,
		"pre-install":  v1alpha1.HookTypePreSync,
		"pre-upgrade":  v1alpha1.HookTypePreSync,
		"post-upgrade": v1alpha1.HookTypePostSync,
		"post-install": v1alpha1.HookTypePostSync,
	}[t]
	return hookType, ok
}

type HookDeletePolicy string

func (p HookDeletePolicy) HookDeletePolicy() (v1alpha1.HookDeletePolicy, bool) {
	policy, ok := map[HookDeletePolicy]v1alpha1.HookDeletePolicy{
		"before-hook-creation": v1alpha1.HookDeletePolicyBeforeHookCreation,
		"hook-succeeded":       v1alpha1.HookDeletePolicyHookSucceeded,
		"hook-failed":          v1alpha1.HookDeletePolicyHookFailed,
	}[p]
	return policy, ok
}

func GetHookTypes(obj *unstructured.Unstructured) []HookType {
	var types []HookType
	for _, text := range resource.GetAnnotationCSVs(obj, "helm.sh/hook") {
		types = append(types, HookType(text))
	}
	return types
}

func GetHookDeletePolicies(obj *unstructured.Unstructured) []HookDeletePolicy {
	var policies []HookDeletePolicy
	for _, text := range resource.GetAnnotationCSVs(obj, "helm.sh/hook-delete-policy") {
		policies = append(policies, HookDeletePolicy(text))
	}
	return policies
}

func GetHookWeight(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()["helm.sh/hook-weight"]
	if ok {
		value, err := strconv.ParseInt(text, 10, 0)
		if err == nil {
			return int(value)
		}
	}
	return 0
}
