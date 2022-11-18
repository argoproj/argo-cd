package generators

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/imdario/mergo"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*MergeGenerator)(nil)

var (
	ErrLessThanTwoGeneratorsInMerge = fmt.Errorf("found less than two generators, Merge requires two or more")
	ErrNoMergeKeys                  = fmt.Errorf("no merge keys were specified, Merge requires at least one")
	ErrNonUniqueParamSets           = fmt.Errorf("the parameters from a generator were not unique by the given mergeKeys, Merge requires all param sets to be unique")
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
func (m *MergeGenerator) getParamSetsForAllGenerators(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([][]map[string]interface{}, error) {
	var paramSets [][]map[string]interface{}
	for _, generator := range generators {
		generatorParamSets, err := m.getParams(generator, appSet)
		if err != nil {
			return nil, err
		}
		// concatenate param lists produced by each generator
		paramSets = append(paramSets, generatorParamSets)
	}
	return paramSets, nil
}

// GenerateParams gets the params produced by the MergeGenerator.
func (m *MergeGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	if appSetGenerator.Merge == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if len(appSetGenerator.Merge.Generators) < 2 {
		return nil, ErrLessThanTwoGeneratorsInMerge
	}

	paramSetsFromGenerators, err := m.getParamSetsForAllGenerators(appSetGenerator.Merge.Generators, appSet)
	if err != nil {
		return nil, err
	}

	baseParamSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSetsFromGenerators[0])
	if err != nil {
		return nil, err
	}

	for _, paramSets := range paramSetsFromGenerators[1:] {
		paramSetsByMergeKey, err := getParamSetsByMergeKey(appSetGenerator.Merge.MergeKeys, paramSets)
		if err != nil {
			return nil, err
		}

		for mergeKeyValue, baseParamSet := range baseParamSetsByMergeKey {
			if overrideParamSet, exists := paramSetsByMergeKey[mergeKeyValue]; exists {

				if appSet.Spec.GoTemplate {
					if err := mergo.Merge(&baseParamSet, overrideParamSet, mergo.WithOverride); err != nil {
						return nil, fmt.Errorf("failed to merge base param set with override param set: %w", err)
					}
					baseParamSetsByMergeKey[mergeKeyValue] = baseParamSet
				} else {
					overriddenParamSet, err := utils.CombineStringMapsAllowDuplicates(baseParamSet, overrideParamSet)
					if err != nil {
						return nil, err
					}
					baseParamSetsByMergeKey[mergeKeyValue] = utils.ConvertToMapStringInterface(overriddenParamSet)
				}
			}
		}
	}

	mergedParamSets := make([]map[string]interface{}, len(baseParamSetsByMergeKey))
	var i = 0
	for _, mergedParamSet := range baseParamSetsByMergeKey {
		mergedParamSets[i] = mergedParamSet
		i += 1
	}

	return mergedParamSets, nil
}

// getParamSetsByMergeKey converts the given list of parameter sets to a map of parameter sets where the key is the
// unique key of the parameter set as determined by the given mergeKeys. If any two parameter sets share the same merge
// key, getParamSetsByMergeKey will throw NonUniqueParamSets.
func getParamSetsByMergeKey(mergeKeys []string, paramSets []map[string]interface{}) (map[string]map[string]interface{}, error) {
	if len(mergeKeys) < 1 {
		return nil, ErrNoMergeKeys
	}

	deDuplicatedMergeKeys := make(map[string]bool, len(mergeKeys))
	for _, mergeKey := range mergeKeys {
		deDuplicatedMergeKeys[mergeKey] = false
	}

	paramSetsByMergeKey := make(map[string]map[string]interface{}, len(paramSets))
	for _, paramSet := range paramSets {
		paramSetKey := make(map[string]interface{})
		for mergeKey := range deDuplicatedMergeKeys {
			paramSetKey[mergeKey] = paramSet[mergeKey]
		}
		paramSetKeyJson, err := json.Marshal(paramSetKey)
		if err != nil {
			return nil, err
		}
		paramSetKeyString := string(paramSetKeyJson)
		if _, exists := paramSetsByMergeKey[paramSetKeyString]; exists {
			return nil, fmt.Errorf("%w. Duplicate key was %s", ErrNonUniqueParamSets, paramSetKeyString)
		}
		paramSetsByMergeKey[paramSetKeyString] = paramSet
	}

	return paramSetsByMergeKey, nil
}

// getParams get the parameters generated by this generator.
func (m *MergeGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {

	var matrix *argoprojiov1alpha1.MatrixGenerator
	if appSetBaseGenerator.Matrix != nil {
		nestedMatrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(appSetBaseGenerator.Matrix)
		if err != nil {
			return nil, err
		}
		if nestedMatrix != nil {
			matrix = nestedMatrix.ToMatrixGenerator()
		}
	}

	var mergeGenerator *argoprojiov1alpha1.MergeGenerator
	if appSetBaseGenerator.Merge != nil {
		nestedMerge, err := argoprojiov1alpha1.ToNestedMergeGenerator(appSetBaseGenerator.Merge)
		if err != nil {
			return nil, err
		}
		if nestedMerge != nil {
			mergeGenerator = nestedMerge.ToMergeGenerator()
		}
	}

	t, err := Transform(
		argoprojiov1alpha1.ApplicationSetGenerator{
			List:                    appSetBaseGenerator.List,
			Clusters:                appSetBaseGenerator.Clusters,
			Git:                     appSetBaseGenerator.Git,
			SCMProvider:             appSetBaseGenerator.SCMProvider,
			ClusterDecisionResource: appSetBaseGenerator.ClusterDecisionResource,
			PullRequest:             appSetBaseGenerator.PullRequest,
			Matrix:                  matrix,
			Merge:                   mergeGenerator,
			Selector:                appSetBaseGenerator.Selector,
		},
		m.supportedGenerators,
		argoprojiov1alpha1.ApplicationSetTemplate{},
		appSet,
		map[string]interface{}{})

	if err != nil {
		return nil, fmt.Errorf("child generator returned an error on parameter generation: %v", err)
	}

	if len(t) == 0 {
		return nil, fmt.Errorf("child generator generated no parameters")
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
		base := &argoprojiov1alpha1.ApplicationSetGenerator{
			List:     r.List,
			Clusters: r.Clusters,
			Git:      r.Git,
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
	} else {
		return NoRequeueAfter
	}

}

// GetTemplate gets the Template field for the MergeGenerator.
func (m *MergeGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Merge.Template
}
