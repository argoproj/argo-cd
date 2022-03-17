package diff

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/glob"
)

// IgnoreDiffConfig holds the ignore difference configurations defined in argo-cm
// as well as in the Application resource.
type IgnoreDiffConfig struct {
	ignores   []v1alpha1.ResourceIgnoreDifferences
	overrides map[string]v1alpha1.ResourceOverride
}

// IgnoreDifference holds the configurations to be used while ignoring differences
// from live and desired states.
type IgnoreDifference struct {
	//JSONPointers is a JSON path list following the format defined in RFC4627 (https://datatracker.ietf.org/doc/html/rfc6902#section-3)
	JSONPointers []string
	//JQPathExpressions is a JQ path list that will be evaludated during the diff process
	JQPathExpressions []string
	// ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
	// desired state defined in the SCM and won't be displayed in diffs
	ManagedFieldsManagers []string
}

// NewIgnoreDiffConfig creates a new NewIgnoreDiffConfig.
func NewIgnoreDiffConfig(ignores []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride) *IgnoreDiffConfig {
	return &IgnoreDiffConfig{
		ignores:   ignores,
		overrides: overrides,
	}
}

// HasIgnoreDifference will verify if the provided resource identifiers have any ignore
// difference configurations associated with them. It will first check if there are
// system level ignore difference configurations for the current group/kind. If so, this
// will be returned taking precedence over Application specific ignore difference
// configurations.
func (i *IgnoreDiffConfig) HasIgnoreDifference(group, kind, name, namespace string) (bool, *IgnoreDifference) {
	ro, ok := i.overrides[fmt.Sprintf("%s/%s", group, kind)]
	if ok {
		return ok, overrideToIgnoreDifference(ro)
	}
	wildOverride, ok := i.overrides["*/*"]
	if ok {
		return ok, overrideToIgnoreDifference(wildOverride)
	}

	ignoresFound := []v1alpha1.ResourceIgnoreDifferences{}
	for _, ignore := range i.ignores {
		if glob.Match(ignore.Group, group) &&
			glob.Match(ignore.Kind, kind) &&
			(ignore.Name == "" || ignore.Name == name) &&
			(ignore.Namespace == "" || ignore.Namespace == namespace) {

			ignoresFound = append(ignoresFound, ignore)

		}
	}
	if len(ignoresFound) == 0 {
		return false, nil
	}
	return true, mergeIgnoreDifferences(ignoresFound)
}

func overrideToIgnoreDifference(override v1alpha1.ResourceOverride) *IgnoreDifference {
	return &IgnoreDifference{
		JSONPointers:          override.IgnoreDifferences.JSONPointers,
		JQPathExpressions:     override.IgnoreDifferences.JQPathExpressions,
		ManagedFieldsManagers: override.IgnoreDifferences.ManagedFieldsManagers,
	}
}

// mergeIgnoreDifferences will merge all ignore differences configurations found for
// a specific resource.
func mergeIgnoreDifferences(ignores []v1alpha1.ResourceIgnoreDifferences) *IgnoreDifference {
	result := &IgnoreDifference{}
	for _, ignore := range ignores {
		result.JQPathExpressions = append(result.JQPathExpressions, ignore.JQPathExpressions...)
		result.JSONPointers = append(result.JSONPointers, ignore.JSONPointers...)
		result.ManagedFieldsManagers = append(result.ManagedFieldsManagers, ignore.ManagedFieldsManagers...)
	}
	return result
}
