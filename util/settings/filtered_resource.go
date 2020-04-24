package settings

import (
	"github.com/gobwas/glob"
	log "github.com/sirupsen/logrus"
)

type FilteredResource struct {
	APIGroups []string            `json:"apiGroups,omitempty"`
	Kinds     []string            `json:"kinds,omitempty"`
	Clusters  []string            `json:"clusters,omitempty"`
	Labels    []map[string]string `json:"labels,omitempty"`
}

func (r FilteredResource) matchGroup(apiGroup string) bool {
	for _, excludedApiGroup := range r.APIGroups {
		if match(excludedApiGroup, apiGroup) {
			return true
		}
	}
	return len(r.APIGroups) == 0
}

func match(pattern, text string) bool {
	compiledGlob, err := glob.Compile(pattern)
	if err != nil {
		log.Warnf("failed to compile pattern %s due to error %v", pattern, err)
		return false
	}
	return compiledGlob.Match(text)
}

func (r FilteredResource) matchKind(kind string) bool {
	for _, excludedKind := range r.Kinds {
		if excludedKind == "*" || excludedKind == kind {
			return true
		}
	}
	return len(r.Kinds) == 0
}

func (r FilteredResource) matchCluster(cluster string) bool {
	for _, excludedCluster := range r.Clusters {
		if match(excludedCluster, cluster) {
			return true
		}
	}
	return len(r.Clusters) == 0
}

func (r FilteredResource) matchLabels(labels map[string]string) bool {
	for _, excludedLabels := range r.Labels {
		for key, specifiedLabel := range excludedLabels {
			if existingLabel, ok := labels[key]; ok {
				if match(specifiedLabel, existingLabel) {
					return true
				}
			}
		}
	}
	return len(r.Labels) == 0
}

func (r FilteredResource) Match(apiGroup, kind, cluster string, labels map[string]string) bool {
	return r.matchGroup(apiGroup) && r.matchKind(kind) && r.matchCluster(cluster) && r.matchLabels(labels)
}
