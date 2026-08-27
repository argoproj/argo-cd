package settings

import "strings"

// The core exclusion list are K8s resources that we assume will never be managed by operators,
// and are never child objects of managed resources that need to be presented in the resource tree.
// This list contains high volume and  high churn metadata objects which we exclude for performance
// reasons, reducing connections and load to the K8s API servers of managed clusters.
var coreExcludedResources = []FilteredResource{
	{APIGroups: []string{"events.k8s.io", "metrics.k8s.io"}},
	{APIGroups: []string{""}, Kinds: []string{"Event"}},
	{APIGroups: []string{"coordination.k8s.io"}, Kinds: []string{"Lease"}},
}

type ResourcesFilter struct {
	// ResourceExclusions holds the api groups, kinds per cluster to exclude from Argo CD's watch
	ResourceExclusions []FilteredResource
	// ResourceInclusions holds the only api groups, kinds per cluster that Argo CD will watch
	ResourceInclusions []FilteredResource
	// ResourceSelectors holds the label selectors, per api group, kind and cluster, that narrow
	// down the objects Argo CD will watch
	ResourceSelectors []FilteredResource
}

func (rf *ResourcesFilter) getExcludedResources() []FilteredResource {
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
func (rf *ResourcesFilter) IsExcludedResource(apiGroup, kind, cluster string) bool {
	// if excluded, do not allow
	if rf.isExcludedResource(apiGroup, kind, cluster) {
		return true
	}

	// if included, do allow
	if rf.isIncludedResource(apiGroup, kind, cluster) {
		return false
	}

	// if inclusion rules defined for cluster, default is not allow
	for _, includedResource := range rf.ResourceInclusions {
		if includedResource.MatchCluster(cluster) {
			return true
		}
	}

	// if no inclusion rules defined for cluster, default is allow
	return false
}

// GetLabelSelector returns the label selector that must be applied to the list/watch calls of the
// given resource so that the API server only returns the objects Argo CD is interested in. The
// selectors of all the matching ResourceSelectors rules are ANDed together. An empty string is
// returned if no rule matches, in which case all the objects of the resource are listed.
func (rf *ResourcesFilter) GetLabelSelector(apiGroup, kind, cluster string) string {
	selectors := make([]string, 0)
	for _, resourceSelector := range rf.ResourceSelectors {
		if resourceSelector.Selector == "" || !resourceSelector.Match(apiGroup, kind, cluster) {
			continue
		}
		selectors = append(selectors, resourceSelector.Selector)
	}
	return strings.Join(selectors, ",")
}
