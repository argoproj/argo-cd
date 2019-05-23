package controller

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	phase          SyncPhase
	liveObj        *unstructured.Unstructured
	targetObj      *unstructured.Unstructured
	skipDryRun     bool
	syncStatus     ResultCode
	operationState OperationPhase
	message        string
}

func str(expr bool, a, b string) string {
	if expr {
		return a
	} else {
		return b
	}
}

func (t *syncTask) String() string {
	return fmt.Sprintf("%sx%d: %s/%s/%s: %s->%s, %s,%s,%s",
		t.phase, t.wave(),
		str(t.isHook(), "hook", "resource"), t.kind(), t.name(),
		str(t.liveObj != nil, "...", "nil"), str(t.targetObj != nil, "...", "nil"),
		t.syncStatus, t.operationState, t.message,
	)
}

func (t *syncTask) isPrune() bool {
	return t.targetObj == nil
}

// return the target object (if this exists) otherwise the live object
// some caution - often you explicitly want the live object not the target object
func (t *syncTask) obj() *unstructured.Unstructured {
	if t.targetObj != nil {
		return t.targetObj
	} else {
		return t.liveObj
	}
}

func (t *syncTask) wave() int {

	text := t.obj().GetAnnotations()["argocd.argoproj.io/sync-wave"]
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
	return isHook(t.obj())
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

func (t *syncTask) running() bool {
	return t.operationState == OperationRunning
}

func (t *syncTask) completed() bool {
	return t.operationState.Completed()
}

func (t *syncTask) successful() bool {
	return t.operationState.Successful()
}
