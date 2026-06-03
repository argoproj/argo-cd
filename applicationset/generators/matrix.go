package generators

import (
	"errors"
	"fmt"
	"time"

	"dario.cat/mergo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*MatrixGenerator)(nil)

var (
	ErrMoreThanTwoGenerators      = errors.New("found more than two generators, Matrix support only two")
	ErrLessThanTwoGenerators      = errors.New("found less than two generators, Matrix support only two")
	ErrMoreThenOneInnerGenerators = errors.New("found more than one generator in matrix.Generators")
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

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, client client.Client) ([]map[string]any, error) {
	return m.generateParams(appSetGenerator, appSet, nil, client)
}

func (m *MatrixGenerator) generateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, inheritedParams map[string]any, client client.Client) ([]map[string]any, error) {
	if appSetGenerator.Matrix == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if len(appSetGenerator.Matrix.Generators) < 2 {
		return nil, ErrLessThanTwoGenerators
	}

	if len(appSetGenerator.Matrix.Generators) > 2 {
		return nil, ErrMoreThanTwoGenerators
	}

	res := []map[string]any{}

	g0, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, inheritedParams, client)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}

	for _, a := range g0 {
		paramsForSecondGenerator, err := combineMatrixParams(a, inheritedParams, appSet.Spec.GoTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to combine params from the first generator in the matrix generator with inherited params: %w", err)
		}

		g1, err := m.getParams(appSetGenerator.Matrix.Generators[1], appSet, paramsForSecondGenerator, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
		}

		for _, b := range g1 {
			val, err := combineMatrixParams(a, b, appSet.Spec.GoTemplate)
			if err != nil {
				return nil, fmt.Errorf("failed to combine params for the matrix generator: %w", err)
			}
			res = append(res, val)
		}
	}

	return res, nil
}

// combineMatrixParams combines two matrix parameter sets.
//
// In GoTemplate mode, preferred takes precedence when both maps contain the same
// key. This keeps the existing matrix precedence rule: params from the first
// child generator win over params from the second child generator.
//
// In non-GoTemplate mode, duplicate keys with different string values are an
// error, matching utils.CombineStringMaps.
func combineMatrixParams(preferred, fallback map[string]any, useGoTemplate bool) (map[string]any, error) {
	if useGoTemplate {
		tmp := map[string]any{}

		if len(fallback) > 0 {
			if err := mergo.Merge(&tmp, fallback, mergo.WithOverride); err != nil {
				return nil, err
			}
		}

		if len(preferred) > 0 {
			if err := mergo.Merge(&tmp, preferred, mergo.WithOverride); err != nil {
				return nil, err
			}
		}

		return tmp, nil
	}

	return utils.CombineStringMaps(preferred, fallback)
}

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, params map[string]any, client client.Client) ([]map[string]any, error) {
	childGenerator, err := nestedGeneratorToApplicationSetGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}

	relevantGenerators := GetRelevantGenerators(childGenerator, m.supportedGenerators)
	if len(relevantGenerators) > 1 {
		return nil, ErrMoreThenOneInnerGenerators
	}

	// Do not call Transform for a nested matrix here. Transform interpolates the
	// whole generator before GenerateParams, but a nested matrix must interpolate
	// each child sequentially so the second child can see params from the first child.
	if childGenerator.Matrix != nil {
		matrixParams, err := m.generateParams(
			&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: childGenerator.Matrix,
			},
			appSet,
			params,
			client,
		)
		if err != nil {
			return nil, fmt.Errorf("child matrix generator returned an error on parameter generation: %w", err)
		}

		return filterParamsBySelector(matrixParams, appSetBaseGenerator.Selector)
	}

	t, err := Transform(
		*childGenerator,
		m.supportedGenerators,
		argoprojiov1alpha1.ApplicationSetTemplate{},
		appSet,
		params,
		client,
	)
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

// nestedGeneratorToApplicationSetGenerator converts an ApplicationSetNestedGenerator
// into an ApplicationSetGenerator so it can be processed by Transform or by MatrixGenerator recursively.
func nestedGeneratorToApplicationSetGenerator(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator) (*argoprojiov1alpha1.ApplicationSetGenerator, error) {
	matrixGen, err := getMatrixGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}

	mergeGen, err := getMergeGenerator(appSetBaseGenerator)
	if err != nil {
		return nil, fmt.Errorf("error retrieving merge generator: %w", err)
	}

	return &argoprojiov1alpha1.ApplicationSetGenerator{
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
	}, nil
}

// filterParamsBySelector filters the given params by the given selector.
func filterParamsBySelector(params []map[string]any, selector *metav1.LabelSelector) ([]map[string]any, error) {
	if selector == nil {
		return params, nil
	}

	parsedSelector, err := utils.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("error parsing label selector: %w", err)
	}

	filteredParams := make([]map[string]any, 0, len(params))
	for _, param := range params {
		flatParam, err := flattenParameters(param)
		if err != nil {
			return nil, fmt.Errorf("error flattening parameters: %w", err)
		}

		if !parsedSelector.Matches(labels.Set(flatParam)) {
			continue
		}

		filteredParams = append(filteredParams, param)
	}

	return filteredParams, nil
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
