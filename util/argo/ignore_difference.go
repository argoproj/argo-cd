package argo

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type IgnoreDiffConfig struct {
	ignores   []v1alpha1.ResourceIgnoreDifferences
	overrides map[string]v1alpha1.ResourceOverride
}

type IgnoreDifference struct {
	//JSONPointers is a JSON path list following the format defined in RFC4627 (https://datatracker.ietf.org/doc/html/rfc6902#section-3)
	JSONPointers []string
	//JQPathExpressions is a JQ path list that will be evaludated during the diff process
	JQPathExpressions []string
	// ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
	// desired state defined in the SCM and won't be displayed in diffs
	ManagedFieldsManagers []string
}

func NewIgnoreDiffConfig(ignores []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride) *IgnoreDiffConfig {
	return &IgnoreDiffConfig{
		ignores:   ignores,
		overrides: overrides,
	}
}

func (i *IgnoreDiffConfig) HasIgnoreDifference(group, kind string) (bool, *IgnoreDifference) {
	ro, ok := i.overrides[fmt.Sprintf("%s/%s", group, kind)]
	if ok {
		return ok, OverrideToIgnoreDifference(ro)
	}

	for _, ignore := range i.ignores {
		if ignore.Group == group && ignore.Kind == kind {
			return true, ResourceToIgnoreDifference(ignore)
		}
	}
	return false, nil
}

func OverrideToIgnoreDifference(override v1alpha1.ResourceOverride) *IgnoreDifference {
	return &IgnoreDifference{
		JSONPointers:          override.IgnoreDifferences.JSONPointers,
		JQPathExpressions:     override.IgnoreDifferences.JQPathExpressions,
		ManagedFieldsManagers: override.IgnoreDifferences.ManagedFieldsManagers,
	}
}

func ResourceToIgnoreDifference(resource v1alpha1.ResourceIgnoreDifferences) *IgnoreDifference {
	return &IgnoreDifference{
		JSONPointers:          resource.JSONPointers,
		JQPathExpressions:     resource.JQPathExpressions,
		ManagedFieldsManagers: resource.ManagedFieldsManagers,
	}
}
