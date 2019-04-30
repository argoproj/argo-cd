package controller

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	liveObj    *unstructured.Unstructured
	targetObj  *unstructured.Unstructured
	skipDryRun bool
}

// resourceOrder represents the correct order of Kubernetes resources within a manifest
var resourceOrder = map[string]int{
	"Namespace":                0,
	"ResourceQuota":            1,
	"LimitRange":               2,
	"PodSecurityPolicy":        3,
	"Secret":                   4,
	"ConfigMap":                5,
	"StorageClass":             6,
	"PersistentVolume":         7,
	"PersistentVolumeClaim":    8,
	"ServiceAccount":           9,
	"CustomResourceDefinition": 10,
	"ClusterRole":              11,
	"ClusterRoleBinding":       12,
	"Role":                     13,
	"RoleBinding":              14,
	"Service":                  15,
	"DaemonSet":                16,
	"Pod":                      17,
	"ReplicationController":    18,
	"ReplicaSet":               19,
	"Deployment":               20,
	"StatefulSet":              21,
	"Job":                      22,
	"CronJob":                  23,
	"Ingress":                  24,
	"APIService":               25,
}

type syncTasks []syncTask

func (s syncTasks) Len() int {
	return len(s)
}

func (s syncTasks) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func getWave(un *unstructured.Unstructured) int {

	text := un.GetAnnotations()["argocd.argoproj.io/sync-wave"]
	if text == "" {
		return 0
	}

	val, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}

	return val
}

func (s syncTasks) Less(i, j int) bool {

	a := s[i].targetObj
	if a == nil {
		return false
	}
	b := s[j].targetObj
	if b == nil {
		return true
	}

	syncWaveA := getWave(a)
	syncWaveB := getWave(b)

	if syncWaveA < syncWaveB {
		return true
	}

	first, aok := resourceOrder[a.GetKind()]
	second, bok := resourceOrder[b.GetKind()]

	// if both are unknown and of different kind sort by kind alphabetically
	if !aok && !bok && a.GetKind() != b.GetKind() {
		return a.GetKind() < b.GetKind()
	}

	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}

	// if same kind (including unknown) sub sort alphanumeric
	if first == second {
		return a.GetName() < b.GetName()
	}
	// sort different kinds
	return first < second
}
