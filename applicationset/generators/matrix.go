package generators

import (
	"fmt"
	"time"

	"github.com/imdario/mergo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"
)

var _ Generator = (*MatrixGenerator)(nil)

var (
	ErrMoreThanTwoGenerators      = fmt.Errorf("found more than two generators, Matrix support only two")
	ErrLessThanTwoGenerators      = fmt.Errorf("found less than two generators, Matrix support only two")
	ErrMoreThenOneInnerGenerators = fmt.Errorf("found more than one generator in matrix.Generators")
)

type MatrixGenerator struct {
	// The inner generators supported by the matrix generator (cluster, git, list...)
	supportedGenerators map[string]Generator
}

func NewMatrixGenerator(supportedGenerators map[string]Generator) Generator {
	m := &MatrixGenerator{
		supportedGenerators: supportedGenerators,
	}
	return m
}

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]interface{}, error) {
	if appSetGenerator.Matrix == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if len(appSetGenerator.Matrix.Generators) < 2 {
		return nil, ErrLessThanTwoGenerators
	}

	if len(appSetGenerator.Matrix.Generators) > 2 {
		return nil, ErrMoreThanTwoGenerators
	}

	res := []map[string]interface{}{}

	g0, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, nil, client)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}
	for _, a := range g0 {
		g1, err := m.getParams(appSetGenerator.Matrix.Generators[1], appSet, a, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
		}
		for _, b := range g1 {
			if appSet.Spec.GoTemplate {
				tmp := map[string]interface{}{}
				if err := mergo.Merge(&tmp, b, mergo.WithOverride); err != nil {
					return nil, fmt.Errorf("failed to merge params from the second generator in the matrix generator with temp map: %w", err)
				}
				if err := mergo.Merge(&tmp, a, mergo.WithOverride); err != nil {
					return nil, fmt.Errorf("failed to merge params from the second generator in the matrix generator with the first: %w", err)
				}
				res = append(res, tmp)
			} else {
				val, err := utils.CombineStringMaps(a, b)
				if err != nil {
					return nil, fmt.Errorf("failed to combine string maps with merging params for the matrix generator: %w", err)
				}
				res = append(res, utils.ConvertToMapStringInterface(val))
			}
		}
	}

	return res, nil
}

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, params map[string]interface{}, client client.Client) ([]map[string]interface{}, error) {
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
		params,
		client)
	if err != nil {
		return nil, fmt.Errorf("child generator returned an error on parameter generation: %w", err)
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
