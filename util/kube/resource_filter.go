package kube

type ResourceFilter interface {
	IsExcludedResource(group, kind, cluster string) bool
	IsIncludedResource(group, kind, cluster string) bool
	IsWhitelistAvailable() bool
}
