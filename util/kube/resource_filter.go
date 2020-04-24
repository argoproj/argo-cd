package kube

type ResourceFilter interface {
	IsExcludedResource(group, kind, cluster string, labels map[string]string) bool
}
