package helm

import (
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	resourceutil "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/resource"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DeletePolicy string

const (
	BeforeHookCreation DeletePolicy = "before-hook-creation"
	HookSucceeded      DeletePolicy = "hook-succeeded"
	HookFailed         DeletePolicy = "hook-failed"
)

// note that we do not take into account if this is or is not a hook, caller should check
func NewDeletePolicy(p string) (DeletePolicy, bool) {
	return DeletePolicy(p), p == string(BeforeHookCreation) || p == string(HookSucceeded) || p == string(HookFailed)
}

var hookDeletePolicies = map[DeletePolicy]common.HookDeletePolicy{
	BeforeHookCreation: common.HookDeletePolicyBeforeHookCreation,
	HookSucceeded:      common.HookDeletePolicyHookSucceeded,
	HookFailed:         common.HookDeletePolicyHookFailed,
}

func (p DeletePolicy) DeletePolicy() common.HookDeletePolicy {
	return hookDeletePolicies[p]
}

func DeletePolicies(obj *unstructured.Unstructured) []DeletePolicy {
	var policies []DeletePolicy
	for _, text := range resourceutil.GetAnnotationCSVs(obj, "helm.sh/hook-delete-policy") {
		p, ok := NewDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	return policies
}
