package kube

type ResourceFilter interface {
	IsExcludedResource(group, kind, cluster string) bool
}
