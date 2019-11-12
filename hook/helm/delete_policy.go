package helm

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/resource"
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

var hookDeletePolicies = map[DeletePolicy]v1alpha1.HookDeletePolicy{
	BeforeHookCreation: v1alpha1.HookDeletePolicyBeforeHookCreation,
	HookSucceeded:      v1alpha1.HookDeletePolicyHookSucceeded,
	HookFailed:         v1alpha1.HookDeletePolicyHookFailed,
}

func (p DeletePolicy) DeletePolicy() v1alpha1.HookDeletePolicy {
	return hookDeletePolicies[p]
}

func DeletePolicies(obj *unstructured.Unstructured) []DeletePolicy {
	var policies []DeletePolicy
	for _, text := range resource.GetAnnotationCSVs(obj, "helm.sh/hook-delete-policy") {
		p, ok := NewDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	return policies
}
