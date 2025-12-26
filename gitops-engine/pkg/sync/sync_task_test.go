package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
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
		{"PreSyncHook", fields{common.SyncPhasePreSync, newHook(common.HookTypePreSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypePreSync},
		{"SyncHook", fields{common.SyncPhaseSync, newHook(common.HookTypeSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypeSync},
		{"PostSyncHook", fields{common.SyncPhasePostSync, newHook(common.HookTypePostSync, common.HookDeletePolicyBeforeHookCreation)}, common.HookTypePostSync},
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
	assert.Equal(t, "Default", (&syncTask{targetObj: testingutils.NewPod()}).waveGroup())
	assert.Equal(t, "1", (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group", "1")}).waveGroup())
	assert.Equal(t, []string{}, (&syncTask{targetObj: testingutils.NewPod()}).waveGroupDependencies())
	assert.Equal(t, []string{"1", "2"}, (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/depends-on", "1,2")}).waveGroupDependencies())
	assert.Equal(t, []string{"1", "2"}, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/depends-on", "1,2"), "argocd.argoproj.io/sync-wave-group", "2")}).waveGroupDependencies())

	var dependencyGraph1 = common.WaveDependencyGraph{Dependencies: map[common.GroupIdentity][]common.GroupIdentity{common.GroupIdentity{Phase: "", WaveGroup: "3"}: []common.GroupIdentity{common.GroupIdentity{Phase: "", WaveGroup: "1"}, common.GroupIdentity{Phase: "", WaveGroup: "2"}}}}
	assert.Equal(t, true, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/depends-on", "1,2"), "argocd.argoproj.io/sync-wave-group", "3")}).dependsOn((&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group", "1")}), dependencyGraph1))
	assert.Equal(t, false, (&syncTask{targetObj: testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/depends-on", "1,2"), "argocd.argoproj.io/sync-wave-group", "3")}).dependsOn((&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group", "4")}), dependencyGraph1))

	var dependencyGraph2 = common.WaveDependencyGraph{Dependencies: map[common.GroupIdentity][]common.GroupIdentity{common.GroupIdentity{Phase: "", WaveGroup: ""}: []common.GroupIdentity{}}}
	assert.Equal(t, true, (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "3")}).dependsOn((&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "2")}), dependencyGraph2))
	assert.Equal(t, false, (&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "3")}).dependsOn((&syncTask{targetObj: testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "4")}), dependencyGraph2))

}
