package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func newHook(hookType common.HookType) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", string(hookType))
}

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
		{"PreSyncHook", fields{common.SyncPhasePreSync, newHook(common.HookTypePreSync)}, common.HookTypePreSync},
		{"SyncHook", fields{common.SyncPhaseSync, newHook(common.HookTypeSync)}, common.HookTypeSync},
		{"PostSyncHook", fields{common.SyncPhasePostSync, newHook(common.HookTypePostSync)}, common.HookTypePostSync},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &syncTask{
				phase:   tt.fields.phase,
				liveObj: tt.fields.liveObj,
			}
			hookType := task.hookType()
			assert.EqualValues(t, tt.want, hookType)
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
