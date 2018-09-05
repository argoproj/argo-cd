package kube

// This code is mostly taken from https://github.com/helm/helm/blob/release-2.10/pkg/tiller/kind_sorter.go

import (
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SortOrder is an ordering of Kinds.
type SortOrder []string

// ResourceOrder represents the correct order of Kubernetes resources within a manifest
var ResourceOrder SortOrder = []string{
	"Namespace",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
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

// SortManifestByKind Sorts manifest by k8s proper creation order
func SortManifestByKind(manifests []*unstructured.Unstructured) []*unstructured.Unstructured {
	ks := newKindSorter(manifests, ResourceOrder)
	sort.Sort(ks)
	return ks.manifests
}

func sortByKind(manifests []*unstructured.Unstructured, ordering SortOrder) []*unstructured.Unstructured {
	ks := newKindSorter(manifests, ordering)
	sort.Sort(ks)
	return ks.manifests
}

type kindSorter struct {
	ordering  map[string]int
	manifests []*unstructured.Unstructured
}

func newKindSorter(m []*unstructured.Unstructured, s SortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		manifests: m,
		ordering:  o,
	}
}

func (k *kindSorter) Len() int { return len(k.manifests) }

func (k *kindSorter) Swap(i, j int) { k.manifests[i], k.manifests[j] = k.manifests[j], k.manifests[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.manifests[i]
	if a == nil {
		return false
	}
	b := k.manifests[j]
	if b == nil {
		return true
	}
	first, aok := k.ordering[a.GetKind()]
	second, bok := k.ordering[b.GetKind()]
	// if same kind (including unknown) sub sort alphanumeric
	if first == second {
		// if both are unknown and of different kind sort by kind alphabetically
		if !aok && !bok && a.GetKind() != b.GetKind() {
			return a.GetKind() < b.GetKind()
		}
		return a.GetName() < b.GetName()
	}
	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}
	// sort different kinds
	return first < second
}
