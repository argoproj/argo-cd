package generators

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
)

const (
	selectorKey = "Selector"
)

type TransformResult struct {
	Params   []map[string]interface{}
	Template map[string]interface{}
}

// Transform a spec generator to list of paramSets and a template
func Transform(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, allGenerators map[string]Generator, baseTemplate *apiextensionsv1.JSON, appSet *argoprojiov1alpha1.ApplicationSet, genParams map[string]interface{}) ([]TransformResult, error) {
	selector, err := metav1.LabelSelectorAsSelector(requestedGenerator.Selector)
	if err != nil {
		return nil, fmt.Errorf("error parsing label selector: %w", err)
	}

	res := []TransformResult{}
	var firstError error
	interpolatedGenerator := requestedGenerator.DeepCopy()

	generators := GetRelevantGenerators(&requestedGenerator, allGenerators)
	for _, g := range generators {
		// we call mergeGeneratorTemplate first because GenerateParams might be more costly so we want to fail fast if there is an error
		mergedTemplate, err := mergeGeneratorTemplate(g, &requestedGenerator, baseTemplate)
		if err != nil {
			log.WithError(err).WithField("generator", g).
				Error("error generating params")
			if firstError == nil {
				firstError = err
			}
			continue
		}
		var params []map[string]interface{}
		if len(genParams) != 0 {
			tempInterpolatedGenerator, err := InterpolateGenerator(&requestedGenerator, genParams, appSet.Spec.GoTemplate)
			interpolatedGenerator = &tempInterpolatedGenerator
			if err != nil {
				log.WithError(err).WithField("genParams", genParams).
					Error("error interpolating params for generator")
				if firstError == nil {
					firstError = err
				}
				continue
			}
		}
		params, err = g.GenerateParams(interpolatedGenerator, appSet)
		if err != nil {
			log.WithError(err).WithField("generator", g).
				Error("error generating params")
			if firstError == nil {
				firstError = err
			}
			continue
		}
		var filterParams []map[string]interface{}
		for _, param := range params {

			if requestedGenerator.Selector != nil && !selector.Matches(labels.Set(keepOnlyStringValues(param))) {
				continue
			}
			filterParams = append(filterParams, param)
		}

		res = append(res, TransformResult{
			Params:   filterParams,
			Template: mergedTemplate,
		})
	}

	return res, firstError
}

func keepOnlyStringValues(in map[string]interface{}) map[string]string {
	var out map[string]string = map[string]string{}

	for key, value := range in {
		if _, ok := value.(string); ok {
			out[key] = value.(string)
		}
	}

	return out
}

func GetRelevantGenerators(requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, generators map[string]Generator) []Generator {
	var res []Generator

	v := reflect.Indirect(reflect.ValueOf(requestedGenerator))
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanInterface() {
			continue
		}
		name := v.Type().Field(i).Name
		if name == selectorKey {
			continue
		}

		if !reflect.ValueOf(field.Interface()).IsNil() {
			res = append(res, generators[name])
		}
	}

	return res
}

func mergeGeneratorTemplate(g Generator, requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetTemplate *apiextensionsv1.JSON) (map[string]interface{}, error) {
	// Make a copy of the value from `GetTemplate()` before merge, rather than copying directly into
	// the provided parameter (which will touch the original resource object returned by client-go)
	dest := g.GetTemplate(requestedGenerator).DeepCopy()

	destMapStringInterface := map[string]interface{}{}

	if dest != nil {
		if err := json.Unmarshal(dest.Raw, &destMapStringInterface); err != nil {
			return nil, err
		}
	}

	applicationSetTemplateMapStringInterface := map[string]interface{}{}

	if err := json.Unmarshal(applicationSetTemplate.Raw, &applicationSetTemplateMapStringInterface); err != nil {
		return nil, err
	}

	err := mergo.Merge(&destMapStringInterface, &applicationSetTemplateMapStringInterface)

	return destMapStringInterface, err
}

// InterpolateGenerator allows interpolating the matrix's 2nd child generator with values from the 1st child generator
// "params" parameter is an array, where each index corresponds to a generator. Each index contains a map w/ that generator's parameters.
func InterpolateGenerator(requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, params map[string]interface{}, useGoTemplate bool) (argoprojiov1alpha1.ApplicationSetGenerator, error) {
	render := utils.Render{}
	interpolatedGenerator, err := render.RenderGeneratorParams(requestedGenerator, params, useGoTemplate)
	if err != nil {
		log.WithError(err).WithField("interpolatedGenerator", interpolatedGenerator).Error("error interpolating generator with other generator's parameter")
		return *interpolatedGenerator, err
	}

	return *interpolatedGenerator, nil
}
