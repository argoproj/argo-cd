package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	helmhook "github.com/argoproj/argo-cd/util/helm/hook"
	"github.com/argoproj/argo-cd/util/resource"
)

func DeletePolicies(obj *unstructured.Unstructured) []v1alpha1.HookDeletePolicy {
	var policies []v1alpha1.HookDeletePolicy
	for _, text := range resource.GetAnnotationCSVs(obj, common.AnnotationKeyHookDeletePolicy) {
		p, ok := v1alpha1.NewHookDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	for _, p := range helmhook.DeletePolicies(obj) {
		policies = append(policies, p.DeletePolicy())
	}
	return policies
}
