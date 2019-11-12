package hook

import (
	"github.com/argoproj/argo-cd/engine/common"
	"github.com/argoproj/argo-cd/engine/hook/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/resource"
)

func DeletePolicies(obj *unstructured.Unstructured) []v1alpha1.HookDeletePolicy {
	var policies []v1alpha1.HookDeletePolicy
	for _, text := range resource.GetAnnotationCSVs(obj, common.AnnotationKeyHookDeletePolicy) {
		p, ok := v1alpha1.NewHookDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	for _, p := range helm.DeletePolicies(obj) {
		policies = append(policies, p.DeletePolicy())
	}
	return policies
}
