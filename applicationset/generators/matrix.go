package generators

import (
	"fmt"
	"time"

	"github.com/imdario/mergo"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {

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

	g0, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, nil)
	if err != nil {
		return nil, err
	}
	for _, a := range g0 {
		g1, err := m.getParams(appSetGenerator.Matrix.Generators[1], appSet, a)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
		}
		for _, b := range g1 {

			if appSet.Spec.GoTemplate {
				tmp := map[string]interface{}{}
				if err := mergo.Merge(&tmp, a); err != nil {
					return nil, fmt.Errorf("failed to merge params from the first generator in the matrix generator with temp map: %w", err)
				}
				if err := mergo.Merge(&tmp, b); err != nil {
					return nil, fmt.Errorf("failed to merge params from the first generator in the matrix generator with the second: %w", err)
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

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet, params map[string]interface{}) ([]map[string]interface{}, error) {
	var matrix *argoprojiov1alpha1.MatrixGenerator
	if appSetBaseGenerator.Matrix != nil {
		// Since nested matrix generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMatrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(appSetBaseGenerator.Matrix)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall nested matrix generator: %v", err)
		}
		if nestedMatrix != nil {
			matrix = nestedMatrix.ToMatrixGenerator()
		}
	}

	var mergeGenerator *argoprojiov1alpha1.MergeGenerator
	if appSetBaseGenerator.Merge != nil {
		// Since nested merge generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMerge, err := argoprojiov1alpha1.ToNestedMergeGenerator(appSetBaseGenerator.Merge)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall nested merge generator: %v", err)
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

func (m *MatrixGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Matrix.Template
}
