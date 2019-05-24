package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hook"
)

func syncPhases(obj *unstructured.Unstructured) []SyncPhase {
	if hook.Skip(obj) {
		return nil
	} else if hook.IsHook(obj) {
		var phases []SyncPhase
		for _, hookType := range hook.HookTypes(obj) {
			switch hookType {
			case HookTypePreSync, HookTypeSync, HookTypePostSync:
				phases = append(phases, SyncPhase(hookType))
			}
		}
		return phases
	} else {
		return []SyncPhase{SyncPhaseSync}
	}
}
