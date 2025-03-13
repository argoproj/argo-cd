package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	assert.Nil(t, DeletePolicies(testingutils.NewPod()))
	assert.Equal(t, []DeletePolicy{BeforeHookCreation}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "before-hook-creation")))
	assert.Equal(t, []DeletePolicy{HookSucceeded}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
	assert.Equal(t, []DeletePolicy{HookFailed}, DeletePolicies(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-delete-policy", "hook-failed")))
}

func TestDeletePolicy_DeletePolicy(t *testing.T) {
	assert.Equal(t, common.HookDeletePolicyBeforeHookCreation, BeforeHookCreation.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookSucceeded, HookSucceeded.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookFailed, HookFailed.DeletePolicy())
}
