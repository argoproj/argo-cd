package controller

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hook"
)

type result struct {
	// whether or not the task is synced
	sync ResultCode
	// if the task ran correctly (maybe have synced ok)
	operation OperationPhase
	message   string
}

func (r result) running() bool {
	return r.operation.Running()
}

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	syncPhase  SyncPhase
	liveObj    *unstructured.Unstructured
	targetObj  *unstructured.Unstructured
	skipDryRun bool
	result     result
}

func newSyncTask(phase SyncPhase, liveObj *unstructured.Unstructured, targetObj *unstructured.Unstructured, skipDryRun bool, result result) syncTask {
	if liveObj == nil && targetObj == nil {
		panic("either liveObj or targetObj must not be nil")
	}
	return syncTask{phase, liveObj, targetObj, skipDryRun, result}
}

func (t *syncTask) String() string {
	return fmt.Sprintf("{syncPhase=%s,wave=%d,kind=%s,name=%s,result=%s}", t.syncPhase, t.wave(), t.kind(), t.name(), t.result)
}

func (t *syncTask) isPrune() bool {
	return t.targetObj == nil
}

func (t *syncTask) getObj() *unstructured.Unstructured {
	if t.targetObj != nil {
		return t.targetObj
	} else {
		return t.liveObj
	}
}

func (t *syncTask) wave() int {

	text := t.getObj().GetAnnotations()["argocd.argoproj.io/sync-wave"]
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
	return hook.IsArgoHook(t.getObj())
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
	return t.getObj().GroupVersionKind()
}

func (t *syncTask) name() string {
	return t.getObj().GetName()
}

func (t *syncTask) namespace() string {
	return t.getObj().GetNamespace()
}
