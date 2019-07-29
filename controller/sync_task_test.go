package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test"
)

func Test_syncTask_hookType(t *testing.T) {
	type fields struct {
		phase   SyncPhase
		liveObj *unstructured.Unstructured
	}
	tests := []struct {
		name   string
		fields fields
		want   HookType
	}{
		{"Empty", fields{SyncPhaseSync, NewPod()}, ""},
		{"PreSyncHook", fields{SyncPhasePreSync, NewHook(HookTypePreSync)}, HookTypePreSync},
		{"SyncHook", fields{SyncPhaseSync, NewHook(HookTypeSync)}, HookTypeSync},
		{"PostSyncHook", fields{SyncPhasePostSync, NewHook(HookTypePostSync)}, HookTypePostSync},
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

func Test_syncTask_wave(t *testing.T) {
	tests := []struct {
		name string
		obj  *unstructured.Unstructured
		want int
	}{
		{"Empty", NewPod(), 0},
		{"SyncWave", Annotate(NewPod(), common.AnnotationSyncWave, "1"), 1},
		{"HookWeight", Annotate(NewPod(), common.AnnotationHelmWeight, "1"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			task := &syncTask{liveObj: tt.obj}
			assert.Equal(t1, tt.want, task.wave())
		})
	}
}

func Test_syncTask_hasHookDeletePolicy(t *testing.T) {
	assert.False(t, (&syncTask{targetObj: NewPod()}).hasHookDeletePolicy(HookDeletePolicyBeforeHookCreation))
	assert.False(t, (&syncTask{targetObj: NewPod()}).hasHookDeletePolicy(HookDeletePolicyHookSucceeded))
	assert.False(t, (&syncTask{targetObj: NewPod()}).hasHookDeletePolicy(HookDeletePolicyHookFailed))
	// must be hook
	assert.False(t, (&syncTask{targetObj: Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).hasHookDeletePolicy(HookDeletePolicyBeforeHookCreation))
	assert.True(t, (&syncTask{targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).hasHookDeletePolicy(HookDeletePolicyBeforeHookCreation))
	assert.True(t, (&syncTask{targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")}).hasHookDeletePolicy(HookDeletePolicyHookSucceeded))
	assert.True(t, (&syncTask{targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookFailed")}).hasHookDeletePolicy(HookDeletePolicyHookFailed))
}

func Test_syncTask_needsDeleting(t *testing.T) {
	assert.False(t, (&syncTask{targetObj: NewPod()}).needsDeleting())
	// must be hook
	assert.False(t, (&syncTask{targetObj: Annotate(NewPod(), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).needsDeleting())
	assert.False(t, (&syncTask{liveObj: Annotate(Annotate(NewPod(), "argqqocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).needsDeleting())
	assert.True(t, (&syncTask{targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).needsDeleting())
	assert.True(t, (&syncTask{targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "BeforeHookCreation")}).needsDeleting())
	assert.True(t, (&syncTask{operationState: OperationSucceeded, targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookSucceeded")}).needsDeleting())
	assert.True(t, (&syncTask{operationState: OperationFailed, targetObj: Annotate(Annotate(NewPod(), "argocd.argoproj.io/hook", "Sync"), "argocd.argoproj.io/hook-delete-policy", "HookFailed")}).needsDeleting())
}
