package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.NewPod()))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "garbage")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyBeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")))
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookFailed}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "HookFailed")))
	// "Never" is a recognized policy, so it is not replaced by the BeforeHookCreation default
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyNever}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "Never")))
	// Helm test
	assert.Equal(t, []common.HookDeletePolicy{common.HookDeletePolicyHookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
}
