package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	t.Parallel()
	assert.Nil(t, DeletePolicies(testingutils.NewPod()))
	assert.Equal(t, []DeletePolicy{BeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "before-hook-creation")))
	assert.Equal(t, []DeletePolicy{HookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
	assert.Equal(t, []DeletePolicy{HookFailed}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-failed")))
}

func TestDeletePolicy_DeletePolicy(t *testing.T) {
	t.Parallel()
	assert.Equal(t, common.HookDeletePolicyBeforeHookCreation, BeforeHookCreation.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookSucceeded, HookSucceeded.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookFailed, HookFailed.DeletePolicy())
}
