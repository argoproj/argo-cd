package controller

import (
	"strings"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

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

type syncTasks []*syncTask

func (s syncTasks) Len() int {
	return len(s)
}

func (s syncTasks) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// order is
// 1. phase
// 2. wave
// 3. kind
// 4. name
func (s syncTasks) Less(i, j int) bool {

	tA := s[i]
	tB := s[j]

	d := syncPhaseOrder[tA.phase] - syncPhaseOrder[tB.phase]
	if d != 0 {
		return d < 0
	}

	d = tA.wave() - tB.wave()
	if d != 0 {
		return d < 0
	}

	a := tA.obj()
	b := tB.obj()

	// we take advantage of the fact that if the kind is not in the kindOrder map,
	// then it will return the default int value of zero, which is the highest value
	d = kindOrder[a.GetKind()] - kindOrder[b.GetKind()]
	if d != 0 {
		return d < 0
	}

	return a.GetName() < b.GetName()
}

func (s syncTasks) Filter(predicate func(task *syncTask) bool) (tasks syncTasks) {
	for _, task := range s {
		if predicate(task) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (s syncTasks) Find(predicate func(task *syncTask) bool) *syncTask {
	for _, task := range s {
		if predicate(task) {
			return task
		}
	}
	return nil
}

func (s syncTasks) String() string {
	var values []string
	for _, task := range s {
		values = append(values, task.String())
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func (s syncTasks) phase() SyncPhase {
	if len(s) > 0 {
		return s[0].phase
	}
	return ""
}

func (s syncTasks) wave() int {
	if len(s) > 0 {
		return s[0].wave()
	}
	return 0
}
