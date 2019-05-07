package controller

import (
	"github.com/argoproj/argo-cd/util/hook"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getSyncPhases(obj *unstructured.Unstructured) (syncPhases []SyncPhase) {
	hookTypes := hook.GetHooks(obj)
	if len(hookTypes) > 0 {
		for _, hookType := range hookTypes {
			switch hookType {
			case HookTypePreSync, HookTypePostSync:
				syncPhases = append(syncPhases, SyncPhase(hookType))
			default:
				syncPhases = append(syncPhases, SyncPhaseSync)
			}
		}
	} else {
		syncPhases = []SyncPhase{SyncPhaseSync}
	}
	return syncPhases
}
