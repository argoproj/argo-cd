package settings

type ResourcesFilter struct {
	// ResourceExclusions holds the api groups, kinds per cluster to exclude from Argo CD's watch
	ResourceExclusions []FilteredResource
	// ResourceInclusions holds the only api groups, kinds per cluster that Argo CD will watch
	ResourceInclusions []FilteredResource
}

func (rf *ResourcesFilter) getExcludedResources() []FilteredResource {
	coreExcludedResources := []FilteredResource{
		{APIGroups: []string{"events.k8s.io", "metrics.k8s.io"}},
		{APIGroups: []string{""}, Kinds: []string{"Event"}},
	}
	return append(coreExcludedResources, rf.ResourceExclusions...)
}

func (rf *ResourcesFilter) checkResourcePresence(apiGroup, kind, cluster string, filteredResources []FilteredResource) bool {

	for _, includedResource := range filteredResources {
		if includedResource.Match(apiGroup, kind, cluster) {
			return true
		}
	}

	return false
}

func (rf *ResourcesFilter) isIncludedResource(apiGroup, kind, cluster string) bool {
	return rf.checkResourcePresence(apiGroup, kind, cluster, rf.ResourceInclusions)
}

func (rf *ResourcesFilter) isExcludedResource(apiGroup, kind, cluster string) bool {
	return rf.checkResourcePresence(apiGroup, kind, cluster, rf.getExcludedResources())
}

// Behavior of this function is as follows:
// +-------------+-------------+-------------+
// |  Inclusions |  Exclusions |    Result   |
// +-------------+-------------+-------------+
// |    Empty    |    Empty    |   Allowed   |
// +-------------+-------------+-------------+
// |   Present   |    Empty    |   Allowed   |
// +-------------+-------------+-------------+
// | Not Present |    Empty    | Not Allowed |
// +-------------+-------------+-------------+
// |    Empty    |   Present   | Not Allowed |
// +-------------+-------------+-------------+
// |    Empty    | Not Present |   Allowed   |
// +-------------+-------------+-------------+
// |   Present   | Not Present |   Allowed   |
// +-------------+-------------+-------------+
// | Not Present |   Present   | Not Allowed |
// +-------------+-------------+-------------+
// | Not Present | Not Present | Not Allowed |
// +-------------+-------------+-------------+
// |   Present   |   Present   | Not Allowed |
// +-------------+-------------+-------------+
//
func (rf *ResourcesFilter) IsExcludedResource(apiGroup, kind, cluster string) bool {
	if len(rf.ResourceInclusions) > 0 {
		if rf.isIncludedResource(apiGroup, kind, cluster) {
			return rf.isExcludedResource(apiGroup, kind, cluster)
		} else {
			return true
		}
	} else {
		return rf.isExcludedResource(apiGroup, kind, cluster)
	}
}
