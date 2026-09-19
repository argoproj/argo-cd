package kube

type ResourceFilter interface {
	IsExcludedResource(group, kind, cluster string) bool
	// GetLabelSelector returns the label selector that must be applied to list/watch calls of the
	// given resource. An empty string means that every object of the resource must be listed.
	GetLabelSelector(group, kind, cluster string) string
}
