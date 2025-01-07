package generators

import (
	"fmt"
	"reflect"

	"github.com/jeremywohl/flatten"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	"k8s.io/apimachinery/pkg/labels"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
)

const (
	selectorKey = "Selector"
)

type TransformResult struct {
	Params   []map[string]interface{}
	Template argoprojiov1alpha1.ApplicationSetTemplate
}

// Transform a spec generator to list of paramSets and a template
func Transform(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, allGenerators map[string]Generator, baseTemplate argoprojiov1alpha1.ApplicationSetTemplate, appSet *argoprojiov1alpha1.ApplicationSet, genParams map[string]interface{}, client client.Client) ([]TransformResult, error) {
	// This is a custom version of the `LabelSelectorAsSelector` that is in k8s.io/apimachinery. This has been copied
	// verbatim from that package, with the difference that we do not have any restrictions on label values. This is done
	// so that, among other things, we can match on cluster urls.
	selector, err := utils.LabelSelectorAsSelector(requestedGenerator.Selector)
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
			tempInterpolatedGenerator, err := InterpolateGenerator(&requestedGenerator, genParams, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
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
		params, err = g.GenerateParams(interpolatedGenerator, appSet, client)
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
			flatParam, err := flattenParameters(param)
			if err != nil {
				log.WithError(err).WithField("generator", g).
					Error("error flattening params")
				if firstError == nil {
					firstError = err
				}
				continue
			}

			if requestedGenerator.Selector != nil && !selector.Matches(labels.Set(flatParam)) {
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

func flattenParameters(in map[string]interface{}) (map[string]string, error) {
	flat, err := flatten.Flatten(in, "", flatten.DotStyle)
	if err != nil {
		return nil, fmt.Errorf("error flatenning parameters: %w", err)
	}

	out := make(map[string]string, len(flat))
	for k, v := range flat {
		out[k] = fmt.Sprintf("%v", v)
	}

	return out, nil
}

func mergeGeneratorTemplate(g Generator, requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetTemplate argoprojiov1alpha1.ApplicationSetTemplate) (argoprojiov1alpha1.ApplicationSetTemplate, error) {
	// Make a copy of the value from `GetTemplate()` before merge, rather than copying directly into
	// the provided parameter (which will touch the original resource object returned by client-go)
	dest := g.GetTemplate(requestedGenerator).DeepCopy()

	err := mergo.Merge(dest, applicationSetTemplate)

	return *dest, err
}

// InterpolateGenerator allows interpolating the matrix's 2nd child generator with values from the 1st child generator
// "params" parameter is an array, where each index corresponds to a generator. Each index contains a map w/ that generator's parameters.
func InterpolateGenerator(requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, params map[string]interface{}, useGoTemplate bool, goTemplateOptions []string) (argoprojiov1alpha1.ApplicationSetGenerator, error) {
	render := utils.Render{}
	interpolatedGenerator, err := render.RenderGeneratorParams(requestedGenerator, params, useGoTemplate, goTemplateOptions)
	if err != nil {
		log.WithError(err).WithField("interpolatedGenerator", interpolatedGenerator).Error("error interpolating generator with other generator's parameter")
		return argoprojiov1alpha1.ApplicationSetGenerator{}, err
	}

	return *interpolatedGenerator, nil
}

// Fixes https://github.com/argoproj/argo-cd/issues/11982 while ensuring backwards compatibility.
// This is only a short-term solution and should be removed in a future major version.
func dropDisabledNestedSelectors(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator) bool {
	var foundSelector bool
	for i := range generators {
		if generators[i].Selector != nil {
			foundSelector = true
			generators[i].Selector = nil
		}
	}
	return foundSelector
}
