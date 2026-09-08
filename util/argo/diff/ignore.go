package diff

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"
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
	// JSONPointers is a JSON path list following the format defined in RFC4627 (https://datatracker.ietf.org/doc/html/rfc6902#section-3)
	JSONPointers []string
	// JQPathExpressions is a JQ path list that will be evaludated during the diff process
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
	result := &IgnoreDifference{}
	found := false
	ro, ok := i.overrides[fmt.Sprintf("%s/%s", group, kind)]
	if ok {
		mergeIgnoreDifferences(overrideToIgnoreDifference(ro), result)
		found = true
	}
	wildOverride, ok := i.overrides["*/*"]
	if ok {
		mergeIgnoreDifferences(overrideToIgnoreDifference(wildOverride), result)
		found = true
	}

	for _, ignore := range i.ignores {
		if glob.Match(ignore.Group, group) &&
			glob.Match(ignore.Kind, kind) &&
			(ignore.Name == "" || ignore.Name == name) &&
			(ignore.Namespace == "" || ignore.Namespace == namespace) {
			mergeIgnoreDifferences(resourceToIgnoreDifference(ignore), result)
			found = true
		}
	}
	if !found {
		return found, nil
	}

	return found, result
}

func overrideToIgnoreDifference(override v1alpha1.ResourceOverride) *IgnoreDifference {
	return &IgnoreDifference{
		JSONPointers:          override.IgnoreDifferences.JSONPointers,
		JQPathExpressions:     override.IgnoreDifferences.JQPathExpressions,
		ManagedFieldsManagers: override.IgnoreDifferences.ManagedFieldsManagers,
	}
}

func resourceToIgnoreDifference(resource v1alpha1.ResourceIgnoreDifferences) *IgnoreDifference {
	return &IgnoreDifference{
		JSONPointers:          resource.JSONPointers,
		JQPathExpressions:     resource.JQPathExpressions,
		ManagedFieldsManagers: resource.ManagedFieldsManagers,
	}
}

// mergeIgnoreDifferences will merge all ignores in the given from in target
// skipping repeated configs.
func mergeIgnoreDifferences(from *IgnoreDifference, target *IgnoreDifference) {
	for _, jqPath := range from.JQPathExpressions {
		if !slices.Contains(target.JQPathExpressions, jqPath) {
			target.JQPathExpressions = append(target.JQPathExpressions, jqPath)
		}
	}
	for _, jsonPointer := range from.JSONPointers {
		if !slices.Contains(target.JSONPointers, jsonPointer) {
			target.JSONPointers = append(target.JSONPointers, jsonPointer)
		}
	}
	for _, manager := range from.ManagedFieldsManagers {
		if !slices.Contains(target.ManagedFieldsManagers, manager) {
			target.ManagedFieldsManagers = append(target.ManagedFieldsManagers, manager)
		}
	}
}

// ExtractIgnoreDifferencesFromAnnotations parses resource annotations to extract ignoreDifferences rules.
// It looks for the argocd.argoproj.io/ignore-differences annotation containing comma-separated JSON pointer paths.
func ExtractIgnoreDifferencesFromAnnotations(resources []*unstructured.Unstructured) []v1alpha1.ResourceIgnoreDifferences {
	var result []v1alpha1.ResourceIgnoreDifferences

	for _, resource := range resources {
		if resource != nil {
			if annotations := resource.GetAnnotations(); annotations != nil {
				ignoreDiffValue, exists := annotations[common.AnnotationKeyIgnoreDifferences]
				if exists && ignoreDiffValue != "" {
					// Parse comma-separated JSON pointer paths
					paths := strings.Split(ignoreDiffValue, ",")
					var jsonPointers []string
					for _, path := range paths {
						trimmed := strings.TrimSpace(path)
						if trimmed != "" {
							jsonPointers = append(jsonPointers, trimmed)
						}
					}
					if len(jsonPointers) > 0 {
						// Create a ResourceIgnoreDifferences entry specific to this resource
						gvk := resource.GroupVersionKind()
						result = append(result, v1alpha1.ResourceIgnoreDifferences{
							Group:        gvk.Group,
							Kind:         gvk.Kind,
							Name:         resource.GetName(),
							Namespace:    resource.GetNamespace(),
							JSONPointers: jsonPointers,
						})
					}
				}
			}
		}
	}

	return result
}

// MergeResourceIgnoreDifferences combines application-level and resource-level ignoreDifferences.
// Resource-level annotations are appended to the application-level rules.
func MergeResourceIgnoreDifferences(appIgnores []v1alpha1.ResourceIgnoreDifferences, resourceIgnores []v1alpha1.ResourceIgnoreDifferences) []v1alpha1.ResourceIgnoreDifferences {
	if len(resourceIgnores) == 0 {
		return appIgnores
	}

	result := make([]v1alpha1.ResourceIgnoreDifferences, 0, len(appIgnores)+len(resourceIgnores))
	result = append(result, appIgnores...)
	result = append(result, resourceIgnores...)
	return result
}
