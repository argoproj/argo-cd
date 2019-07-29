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
		{"NonHookWeight", Annotate(NewPod(), common.AnnotationHelmWeight, "1"), 0},
		{"HookWeight", Annotate(Annotate(NewPod(), common.AnnotationKeyHook, "Sync"), common.AnnotationHelmWeight, "1"), 1},
		{"HelmHookWeight", Annotate(Annotate(NewPod(), common.AnnotationKeyHelmHook, "pre-install"), common.AnnotationHelmWeight, "1"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			task := &syncTask{liveObj: tt.obj}
			assert.Equal(t1, tt.want, task.wave())
		})
	}
}
