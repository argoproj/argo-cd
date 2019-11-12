package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/hook"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
)

func syncPhases(obj *unstructured.Unstructured) []v1alpha1.SyncPhase {
	if hook.Skip(obj) {
		return nil
	} else if hook.IsHook(obj) {
		phasesMap := make(map[v1alpha1.SyncPhase]bool)
		for _, hookType := range hook.Types(obj) {
			switch hookType {
			case v1alpha1.HookTypePreSync, v1alpha1.HookTypeSync, v1alpha1.HookTypePostSync, v1alpha1.HookTypeSyncFail:
				phasesMap[v1alpha1.SyncPhase(hookType)] = true
			}
		}
		var phases []v1alpha1.SyncPhase
		for phase := range phasesMap {
			phases = append(phases, phase)
		}
		return phases
	} else {
		return []v1alpha1.SyncPhase{v1alpha1.SyncPhaseSync}
	}
}
