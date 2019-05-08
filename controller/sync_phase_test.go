package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func Test_getSyncPhases(t *testing.T) {
	tests := []struct {
		name           string
		obj            *unstructured.Unstructured
		wantSyncPhases []SyncPhase
	}{
		{"TestDefault", &unstructured.Unstructured{}, []SyncPhase{SyncPhaseSync}},
		{"TestPreSyncHook", exampleHook("PreSync"), []SyncPhase{SyncPhasePreSync}},
		{"TestSyncHook", exampleHook("Sync"), []SyncPhase{SyncPhaseSync}},
		{"TestSkipHook", exampleHook("Skip"), nil},
		{"TestPostSyncHook", exampleHook("PostSync"), []SyncPhase{SyncPhasePostSync}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantSyncPhases, syncPhases(tt.obj))
		})
	}
}

func exampleHook(hookType string) *unstructured.Unstructured {
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
