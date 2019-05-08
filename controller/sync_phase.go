package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hook"
)

func syncPhases(obj *unstructured.Unstructured) (phases []SyncPhase) {
	hookTypes := hook.Hooks(obj)
	if len(hookTypes) > 0 {
		for _, hookType := range hookTypes {
			switch hookType {
			case HookTypePreSync, HookTypePostSync:
				phases = append(phases, SyncPhase(hookType))
			default:
				phases = append(phases, SyncPhaseSync)
			}
		}
	} else {
		phases = []SyncPhase{SyncPhaseSync}
	}
	return phases
}
