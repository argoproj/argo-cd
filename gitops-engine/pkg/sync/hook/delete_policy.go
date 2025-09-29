package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	helmhook "github.com/argoproj/gitops-engine/pkg/sync/hook/helm"
	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"
)

func DeletePolicies(obj *unstructured.Unstructured) []common.HookDeletePolicy {
	var policies []common.HookDeletePolicy
	for _, text := range resourceutil.GetAnnotationCSVs(obj, common.AnnotationKeyHookDeletePolicy) {
		p, ok := common.NewHookDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	for _, p := range helmhook.DeletePolicies(obj) {
		policies = append(policies, p.DeletePolicy())
	}
	if len(policies) == 0 {
		policies = append(policies, common.HookDeletePolicyBeforeHookCreation)
	}
	return policies
}
