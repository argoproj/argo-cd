package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

func Test_syncTask_hookType(t *testing.T) {
	type fields struct {
		phase   common.SyncPhase
		liveObj *unstructured.Unstructured
	}
	tests := []struct {
		name   string
		fields fields
		want   common.HookType
	}{
		{"Empty", fields{common.SyncPhaseSync, testingutils.NewPod()}, ""},
		{"PreSyncHook", fields{common.SyncPhasePreSync, newHook("hook", common.HookTypePreSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypePreSync},
		{"SyncHook", fields{common.SyncPhaseSync, newHook("hook", common.HookTypeSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypeSync},
		{"PostSyncHook", fields{common.SyncPhasePostSync, newHook("hook", common.HookTypePostSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypePostSync},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &syncTask{
				phase:   tt.fields.phase,
				liveObj: tt.fields.liveObj,
			}
			hookType := task.hookType()
			assert.Equal(t, tt.want, hookType)
		})
	}
}

func Test_syncTask_hasHookDeletePolicy(t *testing.T) {
	assert.False(t, (&syncTask{targetObj: testingutils.NewPod()}).hasHookDeletePolicy(common.HookDeletePolicyBeforeHookCreation))
	assert.False(t, (&syncTask{targetObj: testingutils.NewPod()}).hasHookDeletePolicy(common.HookDeletePolicyHookSucceeded))
	assert.False(t, (&syncTask{targetObj: testingutils.NewPod()}).hasHookDeletePolicy(common.HookDeletePolicyHookFailed))
	// must be hook
	assert.False(t, (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).hasHookDeletePolicy(common.HookDeletePolicyBeforeHookCreation))
	assert.True(t, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).hasHookDeletePolicy(common.HookDeletePolicyBeforeHookCreation))
	assert.True(t, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")}).hasHookDeletePolicy(common.HookDeletePolicyHookSucceeded))
	assert.True(t, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookFailed")}).hasHookDeletePolicy(common.HookDeletePolicyHookFailed))
}

func Test_syncTask_deleteOnPhaseCompletion(t *testing.T) {
	assert.False(t, (&syncTask{liveObj: testingutils.NewPod()}).deleteOnPhaseCompletion())
	// must be hook
	assert.True(t, (&syncTask{operationState: common.OperationSucceeded, liveObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")}).deleteOnPhaseCompletion())
	assert.True(t, (&syncTask{operationState: common.OperationFailed, liveObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookFailed")}).deleteOnPhaseCompletion())
}

// Test_syncTask_deleteOnPhaseResult verifies that the delete policies are driven by the
// outcome of the hook itself and not by the outcome of the sync operation it belongs to.
func Test_syncTask_deleteOnPhaseResult(t *testing.T) {
	hookWithPolicy := func(policy common.HookDeletePolicy) *unstructured.Unstructured {
		return testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", string(policy))
	}
	tests := []struct {
		name           string
		policy         common.HookDeletePolicy
		operationState common.OperationPhase
		want           bool
	}{
		{"HookSucceeded policy, hook succeeded", common.HookDeletePolicyHookSucceeded, common.OperationSucceeded, true},
		{"HookSucceeded policy, hook failed", common.HookDeletePolicyHookSucceeded, common.OperationFailed, false},
		{"HookSucceeded policy, hook still running", common.HookDeletePolicyHookSucceeded, common.OperationRunning, false},
		{"HookSucceeded policy, hook not started", common.HookDeletePolicyHookSucceeded, "", false},
		{"HookFailed policy, hook failed", common.HookDeletePolicyHookFailed, common.OperationFailed, true},
		{"HookFailed policy, hook errored", common.HookDeletePolicyHookFailed, common.OperationError, true},
		{"HookFailed policy, hook succeeded", common.HookDeletePolicyHookFailed, common.OperationSucceeded, false},
		{"HookFailed policy, hook still running", common.HookDeletePolicyHookFailed, common.OperationRunning, false},
		{"HookFailed policy, hook not started", common.HookDeletePolicyHookFailed, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &syncTask{operationState: tt.operationState, liveObj: hookWithPolicy(tt.policy)}
			if tt.policy == common.HookDeletePolicyHookSucceeded {
				assert.Equal(t, tt.want, task.deleteOnPhaseSuccessful())
			} else {
				assert.Equal(t, tt.want, task.deleteOnPhaseFailed())
			}
			assert.Equal(t, tt.want, task.deleteOnPhaseCompletion())
		})
	}
}

func Test_syncTask_deleteBeforeCreation(t *testing.T) {
	assert.False(t, (&syncTask{liveObj: testingutils.NewPod()}).deleteBeforeCreation())
	// must be hook
	assert.False(t, (&syncTask{liveObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).deleteBeforeCreation())
	// no need to delete if no live obj
	assert.False(t, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).deleteBeforeCreation())
	assert.True(t, (&syncTask{liveObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).deleteBeforeCreation())
	assert.True(t, (&syncTask{liveObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).deleteBeforeCreation())
}

func Test_syncTask_wave(t *testing.T) {
	assert.Equal(t, 0, (&syncTask{targetObj: testingutils.NewPod()}).wave())
	assert.Equal(t, 1, (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "1")}).wave())
}
