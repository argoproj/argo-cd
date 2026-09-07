package settings

import "github.com/argoproj/argo-cd/v3/util/glob"

type FilteredResource struct {
	APIGroups []string `json:"apiGroups,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
	// Selector narrows the filter down to the resources whose labels match the selector. The value
	// is a label selector in its string form, e.g. `foo=bar,!baz`, and is passed as is to the
	// list/watch calls. An empty selector matches every resource.
	Selector string `json:"selector,omitempty"`
}

func (r FilteredResource) matchGroup(apiGroup string) bool {
	for _, excludedAPIGroup := range r.APIGroups {
		if glob.Match(excludedAPIGroup, apiGroup) {
			return true
		}
	}
	return len(r.APIGroups) == 0
}

func (r FilteredResource) matchKind(kind string) bool {
	for _, excludedKind := range r.Kinds {
		if excludedKind == "*" || excludedKind == kind {
			return true
		}
	}
	return len(r.Kinds) == 0
}

func (r FilteredResource) MatchCluster(cluster string) bool {
	for _, excludedCluster := range r.Clusters {
		if glob.Match(excludedCluster, cluster) {
			return true
		}
	}
	return len(r.Clusters) == 0
}

func (r FilteredResource) Match(apiGroup, kind, cluster string) bool {
	return r.matchGroup(apiGroup) && r.matchKind(kind) && r.MatchCluster(cluster)
}
