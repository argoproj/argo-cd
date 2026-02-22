package generators

import (
	"errors"
	"fmt"
	"time"

	"dario.cat/mergo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*MatrixGenerator)(nil)

var (
	ErrMaxChildrenEqualsOne       = errors.New("invalid max number of children, value must not equal to one")
	ErrLessThanTwoGenerators      = errors.New("found less than two generators, Matrix support only two")
	ErrMoreThenOneInnerGenerators = errors.New("found more than one generator in matrix.Generators")
	ErrMoreThanMaxGenerators      = errors.New("found more than the max allowed of child generators")
)

type MatrixGenerator struct {
	// The inner generators supported by the matrix generator (cluster, git, list...)
	supportedGenerators map[string]Generator
	MatrixConfig
}
type MatrixConfig struct {
	maxChildren int
}

func NewMatrixConfig(maxChildren int) MatrixConfig {
	return MatrixConfig{
		maxChildren: maxChildren,
	}
}

func NewMatrixGenerator(supportedGenerators map[string]Generator, matrixConfig MatrixConfig) Generator {
	m := &MatrixGenerator{
		supportedGenerators: supportedGenerators,
		MatrixConfig:        matrixConfig,
	}
	return m
}

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	if appSetGenerator.Matrix == nil {
		return nil, ErrEmptyAppSetGenerator
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

	res, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, nil, client)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}

	for i := 1; i < numGens; i++ {
		list := []map[string]any{}
		gen := appSetGenerator.Matrix.Generators[i]
		for _, prevParam := range res {
			params, err := m.getParams(gen, appSet, prevParam, client)
			if err != nil {
				return nil, fmt.Errorf("failed to get params for generator %d in the matrix generator: %w", i, err)
			}
			for _, currParam := range params {
				if appSet.Spec.GoTemplate {
					tmp := map[string]any{}
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
					list = append(list, val)
				}
			}
		}

		res = list
	}

	return res, nil
}

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, params map[string]any, client client.Client) ([]map[string]any, error) {
	matrixGen, err := getMatrixGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}
	mergeGen, err := getMergeGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, fmt.Errorf("error retrieving merge generator: %w", err)
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
		return nil, errors.New("child generator generated no parameters")
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
	}
	return NoRequeueAfter
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
