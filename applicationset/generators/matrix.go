package generators

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"dario.cat/mergo"
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

	gen0 := appSetGenerator.Matrix.Generators[0]
	gen1 := appSetGenerator.Matrix.Generators[1]
	type0, type1 := activeNestedGeneratorTypeName(gen0), activeNestedGeneratorTypeName(gen1)
	sameType := type0 != "" && type0 == type1

	g0, err := m.getParams(gen0, appSet, nil, client)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}
	for _, a := range g0 {
		crossGenParams := a
		if sameType {
			// When both generators are the same type (e.g. ClusterGenerator × ClusterGenerator),
			// restrict cross-generator params to the "values.*" namespace to prevent the first
			// generator's own-namespace keys (e.g. cluster metadata) from pre-resolving the second
			// generator's Values templates. The second generator resolves its own metadata itself.
			crossGenParams = valuesOnlyParams(a)
		}
		g1, err := m.getParams(gen1, appSet, crossGenParams, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
		}
		for _, b := range g1 {
			if appSet.Spec.GoTemplate {
				tmp := map[string]any{}
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
				res = append(res, val)
			}
		}
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

// activeNestedGeneratorTypeName returns the Go struct type name of the active generator field
// inside an ApplicationSetNestedGenerator (e.g. "ClusterGenerator", "GitGenerator").
// Only fields whose element type name ends with "Generator" are considered, which excludes
// Selector (*metav1.LabelSelector) and the raw Matrix/Merge fields (*apiextensionsv1.JSON).
// Returns "" if no matching generator field is set.
func activeNestedGeneratorTypeName(gen argoprojiov1alpha1.ApplicationSetNestedGenerator) string {
	v := reflect.ValueOf(gen)
	t := v.Type()
	for i := range t.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Pointer && !f.IsNil() {
			if name := t.Field(i).Type.Elem().Name(); strings.HasSuffix(name, "Generator") {
				return name
			}
		}
	}
	return ""
}

// valuesOnlyParams returns a shallow copy of params containing only keys in the "values" namespace
// (the "values" key itself and any "values." prefixed flat keys). Used when same-type generators
// are paired in a Matrix to prevent own-namespace key collision during Values pre-resolution.
func valuesOnlyParams(params map[string]any) map[string]any {
	filtered := make(map[string]any, 2)
	for k, v := range params {
		if k == "values" || strings.HasPrefix(k, "values.") {
			filtered[k] = v
		}
	}
	return filtered
}
