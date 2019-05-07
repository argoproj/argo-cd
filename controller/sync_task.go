package controller

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	syncPhase  SyncPhase
	liveObj    *unstructured.Unstructured
	targetObj  *unstructured.Unstructured
	skipDryRun bool
}

func newSyncTask(phase SyncPhase, liveObj *unstructured.Unstructured, targetObj *unstructured.Unstructured, skipDryRun bool) syncTask {
	if liveObj == nil && targetObj == nil {
		panic("either liveObj or targetObj must not be nil")
	}
	return syncTask{phase, liveObj, targetObj, skipDryRun}
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

func (t *syncTask) getWave() int {

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

// kindOrder represents the correct order of Kubernetes resources within a manifest
var syncPhaseOrder = map[SyncPhase]int{
	SyncPhasePreSync:  -1,
	SyncPhaseSync:     0,
	SyncPhasePostSync: 1,
}

// kindOrder represents the correct order of Kubernetes resources within a manifest
var kindOrder = map[string]int{
	"Namespace":                -26,
	"ResourceQuota":            -24,
	"LimitRange":               -24,
	"PodSecurityPolicy":        -23,
	"Secret":                   -22,
	"ConfigMap":                -21,
	"StorageClass":             -20,
	"PersistentVolume":         -19,
	"PersistentVolumeClaim":    -18,
	"ServiceAccount":           -17,
	"CustomResourceDefinition": -16,
	"ClusterRole":              -15,
	"ClusterRoleBinding":       -14,
	"Role":                     -13,
	"RoleBinding":              -12,
	"Service":                  -11,
	"DaemonSet":                -10,
	"Pod":                      -9,
	"ReplicationController":    -8,
	"ReplicaSet":               -7,
	"Deployment":               -6,
	"StatefulSet":              -5,
	"Job":                      -4,
	"CronJob":                  -3,
	"Ingress":                  -2,
	"APIService":               -1,
}

type syncTasks []syncTask

func (s syncTasks) Len() int {
	return len(s)
}

func (s syncTasks) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// order is
// 1. syncPhase
// 2. syncWave
// 3. prune
// 4. kind
// 5. name
func (s syncTasks) Less(i, j int) bool {

	tA := s[i]
	tB := s[j]

	d := syncPhaseOrder[tA.syncPhase] - syncPhaseOrder[tB.syncPhase]
	if d != 0 {
		return d < 0
	}

	d = tA.getWave() - tB.getWave()
	if d != 0 {
		return d < 0
	}

	a := tA.getObj()
	b := tB.getObj()

	d = kindOrder[a.GetKind()] - kindOrder[b.GetKind()]
	if d != 0 {
		return d < 0
	}

	return a.GetName() < b.GetName()
}

func (s syncTasks) Filter(predicate func(t syncTask) bool) (tasks syncTasks) {
	for _, task := range s {
		if predicate(task) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}
