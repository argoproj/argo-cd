package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test"
)

func TestDeletePolicies(t *testing.T) {
	assert.Nil(t, DeletePolicies(NewPod()))
	assert.Nil(t, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "garbage")))
	assert.Equal(t, []HookDeletePolicy{HookDeletePolicyBeforeHookCreation}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")))
	assert.Equal(t, []HookDeletePolicy{HookDeletePolicyHookSucceeded}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")))
	assert.Equal(t, []HookDeletePolicy{HookDeletePolicyHookFailed}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookFailed")))
	// Helm test
	assert.Equal(t, []HookDeletePolicy{HookDeletePolicyHookSucceeded}, DeletePolicies(Annotate(NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
}
