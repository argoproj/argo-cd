package generators

import (
	"fmt"
	"time"

	"github.com/imdario/mergo"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"
)

var _ Generator = (*MatrixGenerator)(nil)

var (
	ErrMaxChildrenEqualsOne       = fmt.Errorf("invalid max number of children, value must not equal to one")
	ErrLessThanTwoGenerators      = fmt.Errorf("found less than two generators, Matrix support only two")
	ErrMoreThenOneInnerGenerators = fmt.Errorf("found more than one generator in matrix.Generators")
	ErrMoreThanMaxGenerators      = fmt.Errorf("found more than the max allowed of child generators")
)

type MatrixGenerator struct {
	// The inner generators supported by the matrix generator (cluster, git, list...)
	supportedGenerators map[string]Generator
	maxChildren         int
}

func NewMatrixGenerator(supportedGenerators map[string]Generator, maxChildren int) Generator {
	m := &MatrixGenerator{
		supportedGenerators: supportedGenerators,
		maxChildren:         maxChildren,
	}
	return m
}

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {

	if appSetGenerator.Matrix == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if m.maxChildren == 1 {
		return nil, ErrMaxChildrenEqualsOne
	}

	numGens := len(appSetGenerator.Matrix.Generators)
	if numGens < 2 {
		return nil, ErrLessThanTwoGenerators
	}

	if m.maxChildren > 1 && numGens > m.maxChildren {
		return nil, ErrMoreThanMaxGenerators
	}

	res, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, nil)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}

	for i := 1; i < numGens; i++ {
		list := []map[string]interface{}{}
		gen := appSetGenerator.Matrix.Generators[i]
		for _, prevParam := range res {
			params, err := m.getParams(gen, appSet, prevParam)
			if err != nil {
				return nil, fmt.Errorf("failed to get params for generator %d in the matrix generator: %w", i, err)
			}
			for _, currParam := range params {
				if appSet.Spec.GoTemplate {
					tmp := map[string]interface{}{}
					if err := mergo.Merge(&tmp, currParam, mergo.WithOverride); err != nil {
						return nil, fmt.Errorf("failed to merge params map from generator %d with temp map in the matrix generator: %w", i, err)
					}
					if err := mergo.Merge(&tmp, prevParam, mergo.WithOverride); err != nil {
						return nil, fmt.Errorf("failed to merge params from generator %d with a previous params map in the matrix generator: %w", i, err)
					}
					list = append(list, tmp)
				} else {
					val, err := utils.CombineStringMaps(prevParam, currParam)
					if err != nil {
						return nil, fmt.Errorf("failed to combine string maps with merging params for the matrix generator: %w", err)
					}
					list = append(list, utils.ConvertToMapStringInterface(val))
				}
			}
		}

		res = list
	}

	return res, nil
}

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, params map[string]interface{}) ([]map[string]interface{}, error) {
	matrixGen, err := getMatrixGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}
	if matrixGen != nil && !appSet.Spec.ApplyNestedSelectors {
		foundSelector := dropDisabledNestedSelectors(matrixGen.Generators)
		if foundSelector {
			log.Warnf("AppSet '%v' defines selector on nested matrix generator's generator without enabling them via 'spec.applyNestedSelectors', ignoring nested selectors", appSet.Name)
		}
	}
	mergeGen, err := getMergeGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, fmt.Errorf("error retrieving merge generator: %w", err)
	}
	if mergeGen != nil && !appSet.Spec.ApplyNestedSelectors {
		foundSelector := dropDisabledNestedSelectors(mergeGen.Generators)
		if foundSelector {
			log.Warnf("AppSet '%v' defines selector on nested merge generator's generator without enabling them via 'spec.applyNestedSelectors', ignoring nested selectors", appSet.Name)
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
			Plugin:                  appSetBaseGenerator.Plugin,
			Matrix:                  matrixGen,
			Merge:                   mergeGen,
			Selector:                appSetBaseGenerator.Selector,
		},
		m.supportedGenerators,
		argoprojiov1alpha1.ApplicationSetTemplate{},
		appSet,
		params)

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

const maxDuration time.Duration = 1<<63 - 1

func (m *MatrixGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	res := maxDuration
	var found bool

	for _, r := range appSetGenerator.Matrix.Generators {
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
	} else {
		return NoRequeueAfter
	}

}

func getMatrixGenerator(r argoprojiov1alpha1.ApplicationSetNestedGenerator) (*argoprojiov1alpha1.MatrixGenerator, error) {
	if r.Matrix == nil {
		return nil, nil
	}
	matrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(r.Matrix)
	if err != nil {
		return nil, err
	}
	return matrix.ToMatrixGenerator(), nil
}

func (m *MatrixGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Matrix.Template
}
