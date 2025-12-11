package sync

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
)

func syncPhases(obj *unstructured.Unstructured) []common.SyncPhase {
	if hook.Skip(obj) {
		return nil
	} else if hook.IsHook(obj) {
		phasesMap := make(map[common.SyncPhase]bool)
		for _, hookType := range hook.Types(obj) {
			switch hookType {
			case common.HookTypePreSync, common.HookTypeSync, common.HookTypePostSync, common.HookTypeSyncFail:
				phasesMap[common.SyncPhase(hookType)] = true
			}
		}
		var phases []common.SyncPhase
		for phase := range phasesMap {
			phases = append(phases, phase)
		}
		return phases
	}
	return []common.SyncPhase{common.SyncPhaseSync}
}
