package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestDeletePolicies(t *testing.T) {
	assert.Nil(t, DeletePolicies(NewPod()))
	assert.Equal(t, []DeletePolicy{BeforeHookCreation}, DeletePolicies(Annotate(NewPod(), "helm.sh/hook-delete-policy", "before-hook-creation")))
	assert.Equal(t, []DeletePolicy{HookSucceeded}, DeletePolicies(Annotate(NewPod(), "helm.sh/hook-delete-policy", "hook-succeeded")))
	assert.Equal(t, []DeletePolicy{HookFailed}, DeletePolicies(Annotate(NewPod(), "helm.sh/hook-delete-policy", "hook-failed")))
}

func TestDeletePolicy_DeletePolicy(t *testing.T) {
	assert.Equal(t, common.HookDeletePolicyBeforeHookCreation, BeforeHookCreation.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookSucceeded, HookSucceeded.DeletePolicy())
	assert.Equal(t, common.HookDeletePolicyHookFailed, HookFailed.DeletePolicy())
}
