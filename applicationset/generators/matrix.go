package generators

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
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

	g0, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet, nil, client)
	if err != nil {
		return nil, fmt.Errorf("error failed to get params for first generator in matrix generator: %w", err)
	}

	// The second generator runs once per parameter set of the first. When its spec holds no
	// template placeholders the outer parameters cannot change what it produces, so it runs once
	// and the rest of the iterations reuse the result. An error here is left for getParams to
	// report inside the loop, with the wrapping it already has.
	reusable := false
	if childSpec, specErr := nestedGeneratorSpec(appSetGenerator.Matrix.Generators[1]); specErr == nil {
		reusable = !specUsesTemplates(childSpec)
	}

	var reusedParams []map[string]any
	if reusable && len(g0) > 0 {
		reusedParams, err = m.getParams(appSetGenerator.Matrix.Generators[1], appSet, g0[0], client)
		if err != nil {
			return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
		}
	}

	for _, a := range g0 {
		var g1 []map[string]any
		if reusable {
			// The merge below writes into the parameters it is given, so each iteration gets a
			// copy rather than the parameters themselves.
			g1 = deepCopyParams(reusedParams)
		} else {
			g1, err = m.getParams(appSetGenerator.Matrix.Generators[1], appSet, a, client)
			if err != nil {
				return nil, fmt.Errorf("failed to get params for second generator in the matrix generator: %w", err)
			}
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
	requestedGenerator, err := nestedGeneratorSpec(appSetBaseGenerator)
	if err != nil {
		return nil, err
	}

	t, err := Transform(
		requestedGenerator,
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

// nestedGeneratorSpec builds the generator that a nested matrix entry describes.
func nestedGeneratorSpec(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator) (argoprojiov1alpha1.ApplicationSetGenerator, error) {
	matrixGen, err := getMatrixGenerator(appSetBaseGenerator)
	if err != nil {
		return argoprojiov1alpha1.ApplicationSetGenerator{}, err
	}
	mergeGen, err := getMergeGenerator(appSetBaseGenerator)
	if err != nil {
		return argoprojiov1alpha1.ApplicationSetGenerator{}, fmt.Errorf("error retrieving merge generator: %w", err)
	}

	return argoprojiov1alpha1.ApplicationSetGenerator{
		List:                    appSetBaseGenerator.List,
		Clusters:                appSetBaseGenerator.Clusters,
		Git:                     appSetBaseGenerator.Git,
		SCMProvider:             appSetBaseGenerator.SCMProvider,
		ClusterDecisionResource: appSetBaseGenerator.ClusterDecisionResource,
		PullRequest:             appSetBaseGenerator.PullRequest,
		Plugin:                  appSetBaseGenerator.Plugin,
		Oci:                     appSetBaseGenerator.Oci,
		Matrix:                  matrixGen,
		Merge:                   mergeGen,
		Selector:                appSetBaseGenerator.Selector,
	}, nil
}

// specUsesTemplates reports whether interpolating the generator with parameters could change it.
// Interpolation only substitutes template placeholders, so a spec holding none is left untouched.
func specUsesTemplates(generator argoprojiov1alpha1.ApplicationSetGenerator) bool {
	raw, err := json.Marshal(generator)
	if err != nil {
		// Cannot tell, so assume the parameters matter.
		return true
	}
	return bytes.Contains(raw, []byte("{{"))
}

func deepCopyParams(params []map[string]any) []map[string]any {
	if params == nil {
		return nil
	}
	out := make([]map[string]any, len(params))
	for i, param := range params {
		out[i] = deepCopyParamMap(param)
	}
	return out
}

func deepCopyParamMap(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = deepCopyParamValue(value)
	}
	return out
}

func deepCopyParamValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return deepCopyParamMap(v)
	case map[string]string:
		return maps.Clone(v)
	case []any:
		// Clone rather than allocate, so a nil list stays nil. A nil list and an empty one
		// serialize differently, and one parameter set must not disagree with the next.
		out := slices.Clone(v)
		for i, item := range out {
			out[i] = deepCopyParamValue(item)
		}
		return out
	case []map[string]any:
		// The cluster generator's flat list mode puts every cluster under one key in this shape,
		// and hands back a nil list when it matched none.
		out := slices.Clone(v)
		for i, item := range out {
			out[i] = deepCopyParamMap(item)
		}
		return out
	case []string:
		return slices.Clone(v)
	default:
		return deepCopyReflected(value)
	}
}

// deepCopyReflected copies maps and slices whose concrete type is not handled above, so that a
// generator producing an unanticipated shape does not end up sharing it between parameter sets.
// Anything else is returned as it is, which is what a scalar needs.
func deepCopyReflected(value any) any {
	original := reflect.ValueOf(value)
	switch original.Kind() {
	case reflect.Map, reflect.Slice:
		// Same reason as the explicit branches above: nil is not the same value as empty, and a
		// nil map or slice has nothing to share anyway.
		if original.IsNil() {
			return value
		}
	}

	switch original.Kind() {
	case reflect.Map:
		out := reflect.MakeMapWithSize(original.Type(), original.Len())
		for iter := original.MapRange(); iter.Next(); {
			out.SetMapIndex(iter.Key(), copiedOrOriginal(iter.Value(), original.Type().Elem()))
		}
		return out.Interface()
	case reflect.Slice:
		out := reflect.MakeSlice(original.Type(), original.Len(), original.Len())
		for i := range original.Len() {
			out.Index(i).Set(copiedOrOriginal(original.Index(i), original.Type().Elem()))
		}
		return out.Interface()
	default:
		return value
	}
}

// copiedOrOriginal deep copies an element, falling back to the element itself when the copy cannot
// be assigned back, which is what a nil interface element gives.
func copiedOrOriginal(element reflect.Value, elementType reflect.Type) reflect.Value {
	copied := reflect.ValueOf(deepCopyParamValue(element.Interface()))
	if !copied.IsValid() || !copied.Type().AssignableTo(elementType) {
		return element
	}
	return copied
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
			Oci:                     r.Oci,
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
