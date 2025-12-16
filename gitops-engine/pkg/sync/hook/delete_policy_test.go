package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.NewPod()))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "garbage")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookFailed}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookFailed")))
	// Helm test
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
}
