package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"time"

	"dario.cat/mergo"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	baseParamSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSetsFromGenerators[0])
	if err != nil {
		return nil, fmt.Errorf("error getting param sets by merge key: %w", err)
	}

	for _, paramSets := range paramSetsFromGenerators[1:] {
		paramSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSets)
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
					maps.Copy(baseParamSet, overrideParamSet)
					baseParamSetsByMergeKey[mergeKeyValue] = baseParamSet
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
// unique key of the parameter set as determined by the given mergeKeys. If any two parameter sets share the same merge
// key, getParamSetsByMergeKey will throw NonUniqueParamSets.
func getParamSetsByMergeKey(mergeKeys []string, paramSets []map[string]any) (map[string]map[string]any, error) {
	if len(mergeKeys) < 1 {
		return nil, ErrNoMergeKeys
	}

	deDuplicatedMergeKeys := make(map[string]bool, len(mergeKeys))
	for _, mergeKey := range mergeKeys {
		deDuplicatedMergeKeys[mergeKey] = false
	}

	paramSetsByMergeKey := make(map[string]map[string]any, len(paramSets))
	for _, paramSet := range paramSets {
		paramSetKey := make(map[string]any)
		for mergeKey := range deDuplicatedMergeKeys {
			paramSetKey[mergeKey] = paramSet[mergeKey]
		}
		paramSetKeyJSON, err := json.Marshal(paramSetKey)
		if err != nil {
			return nil, fmt.Errorf("error marshalling param set key json: %w", err)
		}
		paramSetKeyString := string(paramSetKeyJSON)
		if _, exists := paramSetsByMergeKey[paramSetKeyString]; exists {
			return nil, fmt.Errorf("%w. Duplicate key was %s", ErrNonUniqueParamSets, paramSetKeyString)
		}
		paramSetsByMergeKey[paramSetKeyString] = paramSet
	}

	return paramSetsByMergeKey, nil
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
