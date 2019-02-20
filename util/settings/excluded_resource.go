package settings

import "github.com/gobwas/glob"

type ExcludedResource struct {
	ApiGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
	Clusters  []string `json:"clusters"`
}

func (r ExcludedResource) matchGroup(apiGroup string) bool {
	for _, excludedApiGroup := range r.ApiGroups {
		if glob.MustCompile(excludedApiGroup).Match(apiGroup) {
			return true
		}
	}
	return false
}

func (r ExcludedResource) matchKind(kind string) bool {
	for _, excludedKind := range r.Kinds {
		if excludedKind == "*" || excludedKind == kind {
			return true
		}
	}
	return false
}

func (r ExcludedResource) matchCluster(cluster string) bool {
	for _, excludedCluster := range r.Clusters {
		if glob.MustCompile(excludedCluster).Match(cluster) {
			return true
		}
	}
	return false
}

func (r ExcludedResource) Match(apiGroup, kind, cluster string) bool {
	return r.matchGroup(apiGroup) && r.matchKind(kind) && r.matchCluster(cluster)
}
