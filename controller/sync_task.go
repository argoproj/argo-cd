package controller

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hook"
)

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	phase          v1alpha1.SyncPhase
	liveObj        *unstructured.Unstructured
	targetObj      *unstructured.Unstructured
	skipDryRun     bool
	syncStatus     v1alpha1.ResultCode
	operationState v1alpha1.OperationPhase
	message        string
}

func ternary(val bool, a, b string) string {
	if val {
		return a
	} else {
		return b
	}
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

// return the target object (if this exists) otherwise the live object
// some caution - often you explicitly want the live object not the target object
func (t *syncTask) obj() *unstructured.Unstructured {
	return obj(t.targetObj, t.liveObj)
}

func (t *syncTask) wave() int {

	text := t.obj().GetAnnotations()[common.AnnotationSyncWave]
	if text == "" {
		return 0
	}

	val, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}

	return val
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
	return t.operationState == v1alpha1.OperationRunning
}

func (t *syncTask) completed() bool {
	return t.operationState.Completed()
}

func (t *syncTask) successful() bool {
	return t.operationState.Successful()
}

func (t *syncTask) hookType() v1alpha1.HookType {
	if t.isHook() {
		return v1alpha1.HookType(t.phase)
	} else {
		return ""
	}
}
