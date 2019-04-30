package controller

// This code is mostly taken from https://github.com/helm/helm/blob/release-2.10/pkg/tiller/kind_sorter.go

// sortOrder is an ordering of Kinds.
type sortOrder []string

// resourceOrder represents the correct order of Kubernetes resources within a manifest
var resourceOrder sortOrder = []string{
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

type kindSorter struct {
	ordering  map[string]int
	manifests []syncTask
}

func newKindSorter(m []syncTask, s sortOrder) *kindSorter {
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
	a := k.manifests[i].targetObj
	if a == nil {
		return false
	}
	b := k.manifests[j].targetObj
	if b == nil {
		return true
	}
	first, aok := k.ordering[a.GetKind()]
	second, bok := k.ordering[b.GetKind()]

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
