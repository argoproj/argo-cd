package controller

import (
	"sort"

	"github.com/argoproj/gitops-engine/pkg/sync/syncwaves"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type syncWaveSorter []*unstructured.Unstructured

// kindOrder is the order in which manifests should be uninstalled (by Kind)
// https://github.com/helm/helm/blob/master/pkg/releaseutil/kind_sorter.go
var kindOrder = map[string]int{}

func init() {
	kinds := []string{
		"APIService",
		"Ingress",
		"Service",
		"CronJob",
		"Job",
		"StatefulSet",
		"HorizontalPodAutoscaler",
		"Deployment",
		"ReplicaSet",
		"ReplicationController",
		"Pod",
		"DaemonSet",
		"RoleBindingList",
		"RoleBinding",
		"RoleList",
		"Role",
		"ClusterRoleBindingList",
		"ClusterRoleBinding",
		"ClusterRoleList",
		"ClusterRole",
		"CustomResourceDefinition",
		"PersistentVolumeClaim",
		"PersistentVolume",
		"StorageClass",
		"ConfigMap",
		"SecretList",
		"Secret",
		"ServiceAccount",
		"PodDisruptionBudget",
		"PodSecurityPolicy",
		"LimitRange",
		"ResourceQuota",
		"NetworkPolicy",
		"Namespace",
	}
	for i, kind := range kinds {
		kindOrder[kind] = i
	}
}

func (s syncWaveSorter) Len() int {
	return len(s)
}

func (s syncWaveSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s syncWaveSorter) Less(i, j int) bool {
	a, b := s[i], s[j]
	diff := syncwaves.Wave(a) - syncwaves.Wave(b)
	if diff != 0 {
		return diff < 0
	}

	diff = kindOrder[b.GetKind()] - kindOrder[a.GetKind()]
	if diff != 0 {
		return diff < 0
	}

	return a.GetName() < b.GetName()
}

func FilterObjectsForDeletion(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	if len(objs) <= 1 {
		return objs
	}

	sort.Sort(sort.Reverse(syncWaveSorter(objs)))

	currentSyncWave := syncwaves.Wave(objs[0])
	syncFilteredObjs := make([]*unstructured.Unstructured, 0)
	for _, obj := range objs {
		if syncwaves.Wave(obj) != currentSyncWave {
			break
		}
		syncFilteredObjs = append(syncFilteredObjs, obj)
	}

	currentKind := syncFilteredObjs[0].GetKind()
	kindFilteredObjs := make([]*unstructured.Unstructured, 0)
	for _, obj := range syncFilteredObjs {
		if obj.GetKind() != currentKind {
			break
		}
		kindFilteredObjs = append(kindFilteredObjs, obj)
	}

	return kindFilteredObjs
}
