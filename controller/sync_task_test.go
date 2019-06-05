package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
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
		{"Empty", fields{SyncPhaseSync, test.NewPod()}, ""},
		{"PreSyncHook", fields{SyncPhasePreSync, test.NewHook(HookTypePreSync)}, HookTypePreSync},
		{"SyncHook", fields{SyncPhaseSync, test.NewHook(HookTypeSync)}, HookTypeSync},
		{"PostSyncHook", fields{SyncPhasePostSync, test.NewHook(HookTypePostSync)}, HookTypePostSync},
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
