package controller

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func Test_getSyncPhases(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name           string
		args           args
		wantSyncPhases []SyncPhase
	}{
		{"TestPreSync", args{example("PreSync")}, []SyncPhase{SyncPhasePreSync}},
		{"TestSync", args{example("Sync")}, []SyncPhase{SyncPhaseSync}},
		{"TestSkip", args{example("Skip")}, []SyncPhase{SyncPhaseSync}},
		{"TestPostSync", args{example("PostSync")}, []SyncPhase{SyncPhasePostSync}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotSyncPhases := getSyncPhases(tt.args.obj); !reflect.DeepEqual(gotSyncPhases, tt.wantSyncPhases) {
				t.Errorf("getSyncPhases() = %v, want %v", gotSyncPhases, tt.wantSyncPhases)
			}
		})
	}
}

func example(hookType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"argocd.argoproj.io/hook": hookType,
				},
			},
		},
	}
}
