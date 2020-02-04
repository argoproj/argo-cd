package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	. "github.com/argoproj/argo-cd/engine/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(NewPod()))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "garbage")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookFailed}, DeletePolicies(Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookFailed")))
	// Helm test
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(Annotate(NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
}
