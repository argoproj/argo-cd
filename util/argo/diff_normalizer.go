package argo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/diff"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/qri-io/jsonpointer"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type normalizerPatch struct {
	groupKind  schema.GroupKind
	namespace  string
	name       string
	patch      jsonpatch.Patch
	strategy   string
	conditions []*DiffCondition
}

type normalizer struct {
	patches []normalizerPatch
}

// Fields to unmarshal from ignoreDifferences configuration in argocd-cm
type overrideIgnoreDiff struct {
	// List of JSON pointers to ignore in the diff
	JSONPointers []string `yaml:"jsonPointers"`
	// List of conditions to match against object data so this diff applies
	Conditions []string `yaml:"conditions"`
	// Match strategy (one of "all", "any" or "none")
	Strategy string `yaml:"matchStrategy"`
}

// Some constants used by the condition matching algorithm
const (
	isDefined    = 1
	isNotDefined = 2
)

// Represents a condition that must match in order for a patch to be applied
type DiffCondition struct {
	// JSON pointer where to get the value to match
	jsonPath string
	// Operator to use for the match
	operator int
	// Value that should match
	matcher string
}

// Parse a condition for the diff normalizer from a string.
// Conditions may be in the following form:
//
//   PATH COMPARISOR VALUE
//   PATH is defined
//   PATH not defined
//
// PATH is the JSON Pointer compatible path to the JSON data of the object
//
// The special forms "PATH is defined" and "PATH not defined" just test for the
// existance (or absence) of PATH within the object specification.
//
func parseNormalizerCondition(condition string) (*DiffCondition, error) {
	diffCondition := &DiffCondition{}

	// Each condition must have at least 3 segments
	sp := strings.SplitN(condition, " ", 3)
	if len(sp) != 3 {
		return nil, fmt.Errorf("Syntax error")
	}

	// PATH must be absolute
	if !strings.HasPrefix(sp[0], "/") {
		return nil, fmt.Errorf("Target must be absolute JSON path")
	}

	// Handle special cases 'is defined' and 'not defined'
	if sp[2] == "defined" {
		diffCondition.jsonPath = sp[0]
		if sp[1] == "is" {
			diffCondition.operator = isDefined
		} else if sp[1] == "not" {
			diffCondition.operator = isNotDefined
		} else {
			return nil, fmt.Errorf("Unknown operator: '%s'", strings.Join(sp[1:], " "))
		}
	}

	return diffCondition, nil
}

// This function tries to match a configured condition against the JSON data
// of the live object
func matchCondition(data []byte, condition *DiffCondition) bool {
	parsed := map[string]interface{}{}

	if err := json.Unmarshal(data, &parsed); err != nil {
		log.Warnf("Could not unmarshal JSON data: %v", err)
		return false
	}

	// Validate the JSON pointer
	ptr, err := jsonpointer.Parse(condition.jsonPath)
	if err != nil {
		log.Warnf("Invalid JSON pointer in condition: %s", condition.jsonPath)
		return false
	}

	// Eval sets error if the path cannot be found.
	has, err := ptr.Eval(parsed)
	if err != nil {
		if !strings.Contains(err.Error(), "invalid JSON pointer") {
			log.Warnf("Not matched: %s (%v)", condition.jsonPath, err)
		}
		return false
	}

	// Perform the match on the data returned by the pointer evaluation
	switch condition.operator {
	case isDefined:
		return has != nil
	case isNotDefined:
		return has == nil
	default:
		log.Warnf("Unknown operator in diff condition: %d", condition.operator)
	}

	return false
}

// NewDiffNormalizer creates diff normalizer which removes ignored fields according to given application spec and resource overrides
func NewDiffNormalizer(ignore []v1alpha1.ResourceIgnoreDifferences, overrides map[string]v1alpha1.ResourceOverride) (diff.Normalizer, error) {
	for key, override := range overrides {
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			continue
		}
		group := parts[0]
		kind := parts[1]
		if override.IgnoreDifferences != "" {
			ignoreSettings := overrideIgnoreDiff{}
			err := yaml.Unmarshal([]byte(override.IgnoreDifferences), &ignoreSettings)
			if err != nil {
				return nil, err
			}

			ignore = append(ignore, v1alpha1.ResourceIgnoreDifferences{
				Group:         group,
				Kind:          kind,
				JSONPointers:  ignoreSettings.JSONPointers,
				Conditions:    ignoreSettings.Conditions,
				MatchStrategy: ignoreSettings.Strategy,
			})
		}
	}

	patches := make([]normalizerPatch, 0)
	for i := range ignore {
		// Parse all the conditions that have been defined in the spec and append
		// the results into our list of conditions to match.
		conditions := []*DiffCondition{}
		for _, s := range ignore[i].Conditions {
			condition, err := parseNormalizerCondition(s)
			if err != nil {
				log.Warnf("Could not parse condition '%s': %v", s, err)
			} else {
				conditions = append(conditions, condition)
			}
		}

		// Validate match strategy or set the default one
		switch ignore[i].MatchStrategy {
		case "":
			// The default strategy is match all
			ignore[i].MatchStrategy = "all"
		case "all", "any", "none":
			// above stragies are valid, go switch doesn't fall through
		default:
			// invalid strategy voids the conditional matcher
			log.Warnf("Invalid matchStrategy: %s", ignore[i].MatchStrategy)
			continue
		}

		strategy := ignore[i].MatchStrategy

		for _, path := range ignore[i].JSONPointers {
			patchData, err := json.Marshal([]map[string]string{{"op": "remove", "path": path}})
			if err != nil {
				return nil, err
			}
			patch, err := jsonpatch.DecodePatch(patchData)
			if err != nil {
				return nil, err
			}

			patches = append(patches, normalizerPatch{
				groupKind:  schema.GroupKind{Group: ignore[i].Group, Kind: ignore[i].Kind},
				name:       ignore[i].Name,
				namespace:  ignore[i].Namespace,
				patch:      patch,
				conditions: conditions,
				strategy:   strategy,
			})
		}

	}
	return &normalizer{patches: patches}, nil
}

// Normalize removes fields from supplied resource using json paths from matching items of specified resources ignored differences list
func (n *normalizer) Normalize(un *unstructured.Unstructured) error {
	matched := make([]normalizerPatch, 0)
	for _, patch := range n.patches {
		groupKind := un.GroupVersionKind().GroupKind()
		if groupKind == patch.groupKind &&
			(patch.name == "" || patch.name == un.GetName()) &&
			(patch.namespace == "" || patch.namespace == un.GetNamespace()) {
			matched = append(matched, patch)
		}
	}
	if len(matched) == 0 {
		return nil
	}

	docData, err := json.Marshal(un)
	if err != nil {
		return err
	}

	for _, patch := range matched {
		// The default behaviour is to apply the patch, unless we have conditions
		// defined. Whether the patch is applied is then dependent upon the match
		// strategy. A strategy of "all" requires all conditions to be matched, a
		// strategy of "any" requires just one of the conditions to be matched and
		// finally a strategy of "none" requires that none of the conditions are
		// matched.
		applyPatch := true
		if len(patch.conditions) > 0 {
			if patch.strategy == "none" {
				applyPatch = true
			} else {
				applyPatch = false
			}
			for _, cond := range patch.conditions {
				ok := matchCondition(docData, cond)
				if ok {
					if patch.strategy == "none" {
						// None breaks out false with the first matching condition
						applyPatch = false
						break
					} else if patch.strategy == "any" {
						// Any breaks out true with the first matching condition
						applyPatch = true
						break
					} else {
						applyPatch = true
					}
				} else {
					// All breaks out false with the first non-matching condition
					if patch.strategy == "all" {
						applyPatch = false
						break
					}
				}
			}
		}

		if applyPatch {
			patchedData, err := patch.patch.Apply(docData)
			if err != nil {
				log.Debugf("Failed to apply normalization: %v", err)
				continue
			}
			docData = patchedData
		}
	}

	err = json.Unmarshal(docData, un)
	if err != nil {
		return err
	}
	return nil
}
