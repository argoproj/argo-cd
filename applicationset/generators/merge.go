package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"dario.cat/mergo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*MergeGenerator)(nil)

var (
	ErrLessThanTwoGeneratorsInMerge = errors.New("found less than two generators, Merge requires two or more")
	ErrNoMergeKeys                  = errors.New("no merge keys were specified, Merge requires at least one")
	ErrNonUniqueParamSets           = errors.New("the parameters from a generator were not unique by the given mergeKeys, Merge requires all param sets to be unique")
)

type MergeGenerator struct {
	// The inner generators supported by the merge generator (cluster, git, list...)
	supportedGenerators map[string]Generator
}

// NewMergeGenerator returns a MergeGenerator which allows the given supportedGenerators as child generators.
func NewMergeGenerator(supportedGenerators map[string]Generator) Generator {
	m := &MergeGenerator{
		supportedGenerators: supportedGenerators,
	}
	return m
}

// getParamSetsForAllGenerators generates params for each child generator in a MergeGenerator. Param sets are returned
// in slices ordered according to the order of the given generators.
func (m *MergeGenerator) getParamSetsForAllGenerators(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([][]map[string]any, error) {
	var paramSets [][]map[string]any
	for i, generator := range generators {
		generatorParamSets, err := m.getParams(generator, appSet, client)
		if err != nil {
			return nil, fmt.Errorf("error getting params from generator %d of %d: %w", i+1, len(generators), err)
		}
		// concatenate param lists produced by each generator
		paramSets = append(paramSets, generatorParamSets)
	}
	return paramSets, nil
}

// GenerateParams gets the params produced by the MergeGenerator.
func (m *MergeGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	if appSetGenerator.Merge == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if len(appSetGenerator.Merge.Generators) < 2 {
		return nil, ErrLessThanTwoGeneratorsInMerge
	}

	paramSetsFromGenerators, err := m.getParamSetsForAllGenerators(appSetGenerator.Merge.Generators, appSet, client)
	if err != nil {
		return nil, fmt.Errorf("error getting param sets from generators: %w", err)
	}

	baseParamSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSetsFromGenerators[0], appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
	if err != nil {
		return nil, fmt.Errorf("error getting param sets by merge key: %w", err)
	}

	for _, paramSets := range paramSetsFromGenerators[1:] {
		paramSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSets, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("error getting param sets by merge key: %w", err)
		}

		for mergeKeyValue, baseParamSet := range baseParamSetsByMergeKey {
			if overrideParamSet, exists := paramSetsByMergeKey[mergeKeyValue]; exists {
				if appSet.Spec.GoTemplate {
					if err := mergo.Merge(&baseParamSet, overrideParamSet, mergo.WithOverride); err != nil {
						return nil, fmt.Errorf("error merging base param set with override param set: %w", err)
					}
					baseParamSetsByMergeKey[mergeKeyValue] = baseParamSet
				} else {
					overriddenParamSet, err := utils.CombineStringMapsAllowDuplicates(baseParamSet, overrideParamSet)
					if err != nil {
						return nil, fmt.Errorf("error combining string maps: %w", err)
					}
					baseParamSetsByMergeKey[mergeKeyValue] = utils.ConvertToMapStringInterface(overriddenParamSet)
				}
			}
		}
	}

	mergedParamSets := make([]map[string]any, len(baseParamSetsByMergeKey))
	i := 0
	for _, mergedParamSet := range baseParamSetsByMergeKey {
		mergedParamSets[i] = mergedParamSet
		i++
	}

	return mergedParamSets, nil
}

// getParamSetsByMergeKey converts the given list of parameter sets to a map of parameter sets where the key is the
// unique representation of the parameter set as determined by the given mergeKeys. If any two parameter sets share the same merge
// key, getParamSetsByMergeKey will throw NonUniqueParamSets.
func getParamSetsByMergeKey(mergeKeys []string, paramSets []map[string]any, useGoTemplate bool, goTemplateOptions []string) (map[string]map[string]any, error) {
	if len(mergeKeys) < 1 {
		return nil, ErrNoMergeKeys
	}

	// Just deduplicate the merge keys.
	// A merge key may be just a duplicate, we only need it once.
	deDuplicatedMergeKeys := make(map[string]bool, len(mergeKeys))
	for _, mergeKey := range mergeKeys {
		deDuplicatedMergeKeys[mergeKey] = false
	}

	paramSetsByMergeKey := make(map[string]map[string]any, len(paramSets))
	for i, paramSet := range paramSets {
		paramSetRepr, err := generateParamSetRepr(deDuplicatedMergeKeys, paramSet, useGoTemplate, goTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed generating paramSetRepr for paramSet %v: %w", i, err)
		}

		// Convert it to json and back to a string
		// This is necessary, as the value indexed by this key may be a complex
		// object. We want to index with this representation -> String.
		paramSetReprJSON, err := json.Marshal(paramSetRepr)
		if err != nil {
			return nil, fmt.Errorf("error marshalling param set key json: %w", err)
		}
		paramSetReprString := string(paramSetReprJSON)

		// If this was already in the map, we have a duplicate value with respect
		// to these merge keys and these paramSets are not distinguishable.
		if _, exists := paramSetsByMergeKey[paramSetReprString]; exists {
			return nil, fmt.Errorf("%w. Duplicate key was %s", ErrNonUniqueParamSets, paramSetReprString)
		}
		paramSetsByMergeKey[paramSetReprString] = paramSet
	}

	return paramSetsByMergeKey, nil
}

// To be a valid template key (dot separated variable names) a string must be
//
//	                              Start with a character or _
//		                                        |
//	                                            |    Then be alphnumeric
//	                                            |    or a _
//	                                            |         |
//	                                            |         |    Separate with a dot
//	                                            |         |          |
//	                                            |         |          |      Continue with valid var names
//	                                            |         |          |         |
var validGoTemplateKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)

// generateparamSetRepr uses the keys to generate a representation for a paramSet.
//
// This can be thought of a modulo operation: 'paramSet % keys' leaves a
// representation of the paramSet with respect to the keys.
func generateParamSetRepr(keys map[string]bool, paramSet map[string]any, useGoTemplate bool, goTemplateOptions []string) (paramSetRepr map[string]any, err error) {
	paramSetRepr = make(map[string]any)
	// For each part of these keys, fetch the value from the paramSet
	for mergeKey := range keys {
		if useGoTemplate {
			keyTemplate := mergeKey
			// We treat every merge key as a template. But maybe, it is just a 'bare'
			// key, not containing any golang templating.
			// If so, we just surround the whole key with brackets to make
			// go templating evaluate it.
			// To avoid key collisions (e.g. a string containing a valid map
			// representation), we additionally record the real type of the
			// key used.
			if validGoTemplateKey.MatchString(mergeKey) {
				keyTemplate = fmt.Sprintf("{{ kindOf .%s }}:{{ .%s }}", mergeKey, mergeKey)
			}

			// If by now, a string does not contain at least one {{ and }}, someone
			// probably passed in something that should be a goTemplate, but it is not.
			// An example is ".Values". The user intended for this to be a goTemplate,
			// but it is not auto-converted because it starts with a dot.
			// So it will be evaluated to a static string, which is then the same key
			// across all invocations, throwing a "Duplicate key was ..." error.
			// We can do better here:
			if !strings.Contains(keyTemplate, "{{") && !strings.Contains(keyTemplate, "}}") {
				return nil, fmt.Errorf("merge key is not valid goTemplate, missing curly brackets: %s", keyTemplate)
			}

			// Now, this can be templated into a value with respect to the current paramSet
			templatedMergeKey, err := replaceTemplatedString(keyTemplate, paramSet, useGoTemplate, goTemplateOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to template merge key: %w", err)
			}

			paramSetRepr[mergeKey] = templatedMergeKey
		} else {
			paramSetRepr[mergeKey] = paramSet[mergeKey]
		}
	}

	return paramSetRepr, nil
}

// getParams get the parameters generated by this generator.
func (m *MergeGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	matrixGen, err := getMatrixGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}
	mergeGen, err := getMergeGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}

	t, err := Transform(
		argoprojiov1alpha1.ApplicationSetGenerator{
			List:                    appSetBaseGenerator.List,
			Clusters:                appSetBaseGenerator.Clusters,
			Git:                     appSetBaseGenerator.Git,
			SCMProvider:             appSetBaseGenerator.SCMProvider,
			ClusterDecisionResource: appSetBaseGenerator.ClusterDecisionResource,
			PullRequest:             appSetBaseGenerator.PullRequest,
			Plugin:                  appSetBaseGenerator.Plugin,
			Matrix:                  matrixGen,
			Merge:                   mergeGen,
			Selector:                appSetBaseGenerator.Selector,
		},
		m.supportedGenerators,
		argoprojiov1alpha1.ApplicationSetTemplate{},
		appSet,
		map[string]any{}, client)
	if err != nil {
		return nil, fmt.Errorf("child generator returned an error on parameter generation: %w", err)
	}

	if len(t) == 0 {
		return nil, errors.New("child generator generated no parameters")
	}

	if len(t) > 1 {
		return nil, ErrMoreThenOneInnerGenerators
	}

	return t[0].Params, nil
}

func (m *MergeGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	res := maxDuration
	var found bool

	for _, r := range appSetGenerator.Merge.Generators {
		matrixGen, _ := getMatrixGenerator(r)
		mergeGen, _ := getMergeGenerator(r)
		base := &argoprojiov1alpha1.ApplicationSetGenerator{
			List:                    r.List,
			Clusters:                r.Clusters,
			Git:                     r.Git,
			PullRequest:             r.PullRequest,
			Plugin:                  r.Plugin,
			SCMProvider:             r.SCMProvider,
			ClusterDecisionResource: r.ClusterDecisionResource,
			Matrix:                  matrixGen,
			Merge:                   mergeGen,
		}
		generators := GetRelevantGenerators(base, m.supportedGenerators)

		for _, g := range generators {
			temp := g.GetRequeueAfter(base)
			if temp < res && temp != NoRequeueAfter {
				found = true
				res = temp
			}
		}
	}

	if found {
		return res
	}
	return NoRequeueAfter
}

func getMergeGenerator(r argoprojiov1alpha1.ApplicationSetNestedGenerator) (*argoprojiov1alpha1.MergeGenerator, error) {
	if r.Merge == nil {
		return nil, nil
	}
	merge, err := argoprojiov1alpha1.ToNestedMergeGenerator(r.Merge)
	if err != nil {
		return nil, fmt.Errorf("error converting to nested merge generator: %w", err)
	}
	return merge.ToMergeGenerator(), nil
}

// GetTemplate gets the Template field for the MergeGenerator.
func (m *MergeGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Merge.Template
}
