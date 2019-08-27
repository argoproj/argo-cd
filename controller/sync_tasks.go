package controller

import (
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// kindOrder represents the correct order of Kubernetes resources within a manifest
var syncPhaseOrder = map[v1alpha1.SyncPhase]int{
	v1alpha1.SyncPhasePreSync:  -1,
	v1alpha1.SyncPhaseSync:     0,
	v1alpha1.SyncPhasePostSync: 1,
	v1alpha1.SyncPhaseSyncFail: 2,
}

// kindOrder represents the correct order of Kubernetes resources within a manifest
// https://github.com/helm/helm/blob/master/pkg/tiller/kind_sorter.go
var kindOrder = map[string]int{}

func init() {
	kinds := []string{
		"Namespace",
		"ResourceQuota",
		"LimitRange",
		"PodSecurityPolicy",
		"PodDisruptionBudget",
		"Secret",
		"ConfigMap",
		"StorageClass",
		"PersistentVolume",
		"PersistentVolumeClaim",
		"ServiceAccount",
		"CustomResourceDefinition",
		"ClusterRole",
		"ClusterRoleBinding",
		"Role",
		"RoleBinding",
		"Service",
		"DaemonSet",
		"Pod",
		"ReplicationController",
		"ReplicaSet",
		"Deployment",
		"StatefulSet",
		"Job",
		"CronJob",
		"Ingress",
		"APIService",
	}
	for i, kind := range kinds {
		// make sure none of the above entries are zero, we need that for custom resources
		kindOrder[kind] = i - len(kinds)
	}
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

func (s syncTasks) Split(predicate func(task *syncTask) bool) (trueTasks, falseTasks syncTasks) {
	for _, task := range s {
		if predicate(task) {
			trueTasks = append(trueTasks, task)
		} else {
			falseTasks = append(falseTasks, task)
		}
	}
	return trueTasks, falseTasks
}

func (s syncTasks) All(predicate func(task *syncTask) bool) bool {
	for _, task := range s {
		if !predicate(task) {
			return false
		}
	}
	return true
}

func (s syncTasks) Any(predicate func(task *syncTask) bool) bool {
	for _, task := range s {
		if predicate(task) {
			return true
		}
	}
	return false
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

func (s syncTasks) phase() v1alpha1.SyncPhase {
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

func (s syncTasks) lastPhase() v1alpha1.SyncPhase {
	if len(s) > 0 {
		return s[len(s)-1].phase
	}
	return ""
}

func (s syncTasks) lastWave() int {
	if len(s) > 0 {
		return s[len(s)-1].wave()
	}
	return 0
}

func (s syncTasks) multiStep() bool {
	return s.wave() != s.lastWave() || s.phase() != s.lastPhase()
}
