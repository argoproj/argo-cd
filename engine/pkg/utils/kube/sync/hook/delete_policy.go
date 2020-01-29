package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	synccommon "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	helmhook "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/hook/helm"
	resourceutil "github.com/argoproj/argo-cd/engine/pkg/utils/resource"
)

func DeletePolicies(obj *unstructured.Unstructured) []synccommon.HookDeletePolicy {
	var policies []synccommon.HookDeletePolicy
	for _, text := range resourceutil.GetAnnotationCSVs(obj, common.AnnotationKeyHookDeletePolicy) {
		p, ok := synccommon.NewHookDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	for _, p := range helmhook.DeletePolicies(obj) {
		policies = append(policies, p.DeletePolicy())
	}
	if len(policies) == 0 {
		policies = append(policies, synccommon.HookDeletePolicyBeforeHookCreation)
	}
	return policies
}
