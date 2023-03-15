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
	ErrNonUniqueParamsSets          = fmt.Errorf("the parameters from a generator were not unique by the given mergeKeys, Merge requires all param sets to be unique")
)

// Params are a combination of keys and values. Each parameter key (= parameter name)
// needs to be unique to identify a single parameter value.
type Params = map[string]interface{}

// A parameter set is a set of unique parameters
// Note that this is only true for this generator, other generators might not
// require uniqueness for their parameter sets
type ParamsSet = []Params

// A parameter set list is a list (=non unique) of parameter sets
type ParamsSetList = []ParamsSet

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

// GenerateParams gets the params produced by the MergeGenerator.
func (m *MergeGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) (ParamsSet, error) {
	if appSetGenerator.Merge == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if len(appSetGenerator.Merge.Generators) < 2 {
		return nil, ErrLessThanTwoGeneratorsInMerge
	}

	// Evaluate all parameters for the child generators
	paramsSetListFromGenerators, err := m.getParamsSetListForAllGenerators(appSetGenerator.Merge.Generators, appSet)
	if err != nil {
		return nil, err
	}

	// Turn all those sets of parameters into a map indexed by the merge key containing the sets
	paramsSetByMergeKeyList, err := paramsSetListIntoMapsByMergeKey(paramsSetListFromGenerators, appSetGenerator.Merge.MergeKeys)
	if err != nil {
		return nil, fmt.Errorf("failed evaluating merge keys for parameter sets: %e", err)
	}

	// If no parameters have been found (by mergeKey), then no additional parameters
	// can match the base parameters set (and would merge with it). Return the
	// parameters for the base generator then
	baseParamsSetByMergeKey := paramsSetByMergeKeyList[0]
	if len(baseParamsSetByMergeKey) == 0 {
		return paramsSetListFromGenerators[0], nil
	}

	// Merge additional parameter sets into the base parameter set
	additionalParamsSetByMergeKeyList := paramsSetByMergeKeyList[1:]
	err = mergeIntoBaseParamsSet(baseParamsSetByMergeKey, additionalParamsSetByMergeKeyList, appSet.Spec.GoTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to merge additional parameters into base parameter set: %e", err)
	}

	// Turn the indexed baseParamsByMergeKey back into a simple ParamsSet
	mergedParamsSets := make(ParamsSet, len(baseParamsSetByMergeKey))
	var i = 0
	for _, mergedParamsSet := range baseParamsSetByMergeKey {
		mergedParamsSets[i] = mergedParamsSet
		i += 1
	}

	return mergedParamsSets, nil
}

// paramsSetListIntoIndexByMergeKey takes a list of parameter sets and turns them
// into maps containing those lists and indexed by their merge keys
func paramsSetListIntoMapsByMergeKey(paramsSetList ParamsSetList, mergeKeys []string) ([]map[string]Params, error) {
	indexedParamsSetList := make([]map[string]Params, 0)

	for _, paramsSet := range paramsSetList {
		indexedParamsSet, err := indexParamsSetByMergeKeys(mergeKeys, paramsSet)
		if err != nil {
			return nil, err
		}

		indexedParamsSetList = append(indexedParamsSetList, indexedParamsSet)
	}

	return indexedParamsSetList, nil
}

/// mergeIntoBaseParamsSet takes overrides in `additionalParamsSetList` and merges
// those via `mergeKeys` into the `baseParamsByMergeKey`
func mergeIntoBaseParamsSet(baseParamsByMergeKey map[string]Params, additionalParamsSetByMergeKeyList []map[string]Params, goTemplate bool) error {
	for _, additionalParamsByMergeKey := range additionalParamsSetByMergeKeyList {
		// Now merge the additional params into every base parameters map
		for mergeKeyValue, baseParam := range baseParamsByMergeKey {
			// Check if there is a matching override from the additionalParameters
			if overrideParamsSet, exists := additionalParamsByMergeKey[mergeKeyValue]; exists {

				// If we merge with go templates, the parameters might be a nested object,
				// use mergo
				if goTemplate {
					if err := mergo.Merge(&baseParam, overrideParamsSet, mergo.WithOverride); err != nil {
						return fmt.Errorf("failed to merge base param set with override param set: %w", err)
					}
					baseParamsByMergeKey[mergeKeyValue] = baseParam

					// Otherwise combine strings
				} else {
					overriddenParamsSet := utils.CombineStringMapsAllowDuplicates(baseParam, overrideParamsSet)
					baseParamsByMergeKey[mergeKeyValue] = utils.ConvertToMapStringInterface(overriddenParamsSet)
				}
			}
		}
	}

	return nil
}

// getParamsSetListForAllGenerators generates params for each child generator in a MergeGenerator. Param sets are returned
// in slices ordered according to the order of the given generators.
func (m *MergeGenerator) getParamsSetListForAllGenerators(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet) (ParamsSetList, error) {
	var paramsSets ParamsSetList
	for _, generator := range generators {
		generatorParamsSets, err := m.getParamsSet(generator, appSet)
		if err != nil {
			return nil, err
		}
		// concatenate params sets produced by each generator
		paramsSets = append(paramsSets, generatorParamsSets)
	}
	return paramsSets, nil
}

// getParamsSet gets the parameters generated by this generator.
func (m *MergeGenerator) getParamsSet(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet) (ParamsSet, error) {
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
			Matrix:                  matrixGen,
			Merge:                   mergeGen,
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

// indexParamsSetByMergeKeys converts the given set of parameters to a map of
// parameters sets where the key is the unique id of the parameters set as
// determined by the given mergeKeys.
//
// If any two parameters sets share the same values
// in their fields indiciated by mergeKeys, indexParamsSetByMergeKeys will throw NonUniqueParamsSets.
func indexParamsSetByMergeKeys(mergeKeys []string, paramsSet ParamsSet) (map[string]Params, error) {
	if len(mergeKeys) < 1 {
		return nil, ErrNoMergeKeys
	}

	// Deduplicate the merge keys
	deDuplicatedMergeKeys := make(map[string]bool, len(mergeKeys))
	for _, mergeKey := range mergeKeys {
		deDuplicatedMergeKeys[mergeKey] = false
	}

	// All parameters sets indexed by their merge key
	paramsSetsByMergeKey := make(map[string]Params, len(paramsSet))

	for _, paramsSet := range paramsSet {

		// Get the paramsSetId for this paramsSet
		paramsSetId, err := paramsIdByMergeKeys(paramsSet, deDuplicatedMergeKeys)

		// If error, throw
		if err != nil {
			return nil, err
		}

		// If the index is empty, try the next param set, this one is missing at
		// least one mergeKey.
		if paramsSetId == "" {
			continue
		}

		// Otherwise, insert into the paramsSetsByMergeKey, checking for duplicates first
		if _, exists := paramsSetsByMergeKey[paramsSetId]; exists {
			return nil, fmt.Errorf("%w. Duplicate key was %s", ErrNonUniqueParamsSets, paramsSetId)
		}
		paramsSetsByMergeKey[paramsSetId] = paramsSet
	}

	return paramsSetsByMergeKey, nil
}

// paramsIdByMergeKeys computes a single string value from a parameters set
// filtered by some merge keys.
// It is basically a way to compute a "hash" over `params`,
// but only using fields indicated in `mergeKeys`.
//
// If no matches where found, this returns "", nil. In other words, it is required
// that all keys in `mergeKeys` are also in `params` to get a valid id.
func paramsIdByMergeKeys(params Params, mergeKeys map[string]bool) (id string, err error) {
	idMap := make(Params)

	// Try to find values for each merge key in the parameters set
	for mergeKey := range mergeKeys {
		if val, ok := params[mergeKey]; ok {
			idMap[mergeKey] = val
		} else {
			// params is missing a key to merge on
			return "", nil
		}
	}

	// Produce a string from the computed id
	idBytes, err := json.Marshal(idMap)
	if err != nil {
		return "", err
	}
	id = string(idBytes)

	return id, nil
}

func (m *MergeGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	res := maxDuration
	var found bool

	for _, r := range appSetGenerator.Merge.Generators {
		matrixGen, _ := getMatrixGenerator(r)
		mergeGen, _ := getMergeGenerator(r)
		base := &argoprojiov1alpha1.ApplicationSetGenerator{
			List:        r.List,
			Clusters:    r.Clusters,
			Git:         r.Git,
			PullRequest: r.PullRequest,
			Matrix:      matrixGen,
			Merge:       mergeGen,
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

func getMergeGenerator(r argoprojiov1alpha1.ApplicationSetNestedGenerator) (*argoprojiov1alpha1.MergeGenerator, error) {
	if r.Merge == nil {
		return nil, nil
	}
	merge, err := argoprojiov1alpha1.ToNestedMergeGenerator(r.Merge)
	if err != nil {
		return nil, err
	}
	return merge.ToMergeGenerator(), nil
}

// GetTemplate gets the Template field for the MergeGenerator.
func (m *MergeGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Merge.Template
}
