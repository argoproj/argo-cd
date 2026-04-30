package kube

import (
	"sort"

	"k8s.io/kube-openapi/pkg/util/proto"
)

// mergedModels implements proto.Models by delegating lookups across multiple
// per-GroupVersion proto.Models instances. This is needed for OpenAPI v3, where
// each GroupVersion's schema is fetched separately and produces its own Models.
type mergedModels struct {
	all []proto.Models
}

func (m *mergedModels) LookupModel(name string) proto.Schema {
	for _, models := range m.all {
		if s := models.LookupModel(name); s != nil {
			return s
		}
	}
	return nil
}

func (m *mergedModels) ListModels() []string {
	seen := map[string]struct{}{}
	var result []string
	for _, models := range m.all {
		for _, name := range models.ListModels() {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				result = append(result, name)
			}
		}
	}
	sort.Strings(result)
	return result
}

func (m *mergedModels) add(models proto.Models) {
	m.all = append(m.all, models)
}
