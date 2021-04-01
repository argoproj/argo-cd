package settings

import "github.com/argoproj/argo-cd/v2/util/glob"

type FilteredResource struct {
	APIGroups []string `json:"apiGroups,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
}

func (r FilteredResource) matchGroup(apiGroup string) bool {
	for _, excludedApiGroup := range r.APIGroups {
		if glob.Match(excludedApiGroup, apiGroup) {
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
