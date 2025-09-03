package sync

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/syncwaves"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	phase          common.SyncPhase
	liveObj        *unstructured.Unstructured
	targetObj      *unstructured.Unstructured
	skipDryRun     bool
	syncStatus     common.ResultCode
	operationState common.OperationPhase
	message        string
	waveOverride   *int
}

func ternary(val bool, a, b string) string {
	if val {
		return a
	}
	return b
}

func (t *syncTask) String() string {
	return fmt.Sprintf("%s/%d %s %s/%s:%s/%s %s->%s (%s,%s,%s)",
		t.phase, t.wave(),
		ternary(t.isHook(), "hook", "resource"), t.group(), t.kind(), t.namespace(), t.name(),
		ternary(t.liveObj != nil, "obj", "nil"), ternary(t.targetObj != nil, "obj", "nil"),
		t.syncStatus, t.operationState, t.message,
	)
}

func (t *syncTask) isPrune() bool {
	return t.targetObj == nil
}

func (t *syncTask) resultKey() string {
	return resourceResultKey(kube.GetResourceKey(t.obj()), t.phase)
}

// return the target object (if this exists) otherwise the live object
// some caution - often you explicitly want the live object not the target object
func (t *syncTask) obj() *unstructured.Unstructured {
	return obj(t.targetObj, t.liveObj)
}

func (t *syncTask) wave() int {
	if t.waveOverride != nil {
		return *t.waveOverride
	}
	return syncwaves.Wave(t.obj())
}

func (t *syncTask) isHook() bool {
	return hook.IsHook(t.obj())
}

func (t *syncTask) group() string {
	return t.groupVersionKind().Group
}

func (t *syncTask) kind() string {
	return t.groupVersionKind().Kind
}

func (t *syncTask) version() string {
	return t.groupVersionKind().Version
}

func (t *syncTask) groupVersionKind() schema.GroupVersionKind {
	return t.obj().GroupVersionKind()
}

func (t *syncTask) name() string {
	return t.obj().GetName()
}

func (t *syncTask) namespace() string {
	return t.obj().GetNamespace()
}

func (t *syncTask) pending() bool {
	return t.operationState == ""
}

func (t *syncTask) running() bool {
	return t.operationState.Running()
}

func (t *syncTask) completed() bool {
	return t.operationState.Completed()
}

func (t *syncTask) successful() bool {
	return t.operationState.Successful()
}

func (t *syncTask) pruned() bool {
	return t.syncStatus == common.ResultCodePruned
}

func (t *syncTask) hookType() common.HookType {
	if t.isHook() {
		return common.HookType(t.phase)
	}
	return ""
}

func (t *syncTask) hasHookDeletePolicy(policy common.HookDeletePolicy) bool {
	// cannot have a policy if it is not a hook, it is meaningless
	if !t.isHook() {
		return false
	}
	for _, p := range hook.DeletePolicies(t.obj()) {
		if p == policy {
			return true
		}
	}
	return false
}

func (t *syncTask) deleteBeforeCreation() bool {
	return t.liveObj != nil && t.pending() && t.hasHookDeletePolicy(common.HookDeletePolicyBeforeHookCreation)
}

func (t *syncTask) deleteOnPhaseCompletion() bool {
	return t.deleteOnPhaseFailed() || t.deleteOnPhaseSuccessful()
}

func (t *syncTask) deleteOnPhaseSuccessful() bool {
	return t.liveObj != nil && t.hasHookDeletePolicy(common.HookDeletePolicyHookSucceeded)
}

func (t *syncTask) deleteOnPhaseFailed() bool {
	return t.liveObj != nil && t.hasHookDeletePolicy(common.HookDeletePolicyHookFailed)
}

func (t *syncTask) resourceKey() kube.ResourceKey {
	return kube.GetResourceKey(t.obj())
}
