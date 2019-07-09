package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hook"
)

func syncPhases(obj *unstructured.Unstructured) []v1alpha1.SyncPhase {
	if hook.Skip(obj) {
		return nil
	} else if hook.IsHook(obj) {
		var phases []v1alpha1.SyncPhase
		for _, hookType := range hook.Types(obj) {
			switch hookType {
			case v1alpha1.HookTypePreSync, v1alpha1.HookTypeSync, v1alpha1.HookTypePostSync, v1alpha1.HookTypeSyncFail:
				phases = append(phases, v1alpha1.SyncPhase(hookType))
			}
		}
		return phases
	} else {
		return []v1alpha1.SyncPhase{v1alpha1.SyncPhaseSync}
	}
}
