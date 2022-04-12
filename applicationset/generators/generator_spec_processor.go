package generators

import (
	"encoding/json"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/valyala/fasttemplate"
	"reflect"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
)

type TransformResult struct {
	Params   []map[string]string
	Template argoprojiov1alpha1.ApplicationSetTemplate
}

//Transform a spec generator to list of paramSets and a template
func Transform(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, allGenerators map[string]Generator, baseTemplate argoprojiov1alpha1.ApplicationSetTemplate, appSet *argoprojiov1alpha1.ApplicationSet, genParams []map[string]string) ([]TransformResult, error) {
	res := []TransformResult{}
	var firstError error

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
		if len(genParams) != 0 {
			err := interpolateGenerator(&requestedGenerator, genParams)
			if err != nil {
				log.WithError(err).WithField("genParams", genParams).
					Error("error interpolating params for generator")
				if firstError == nil {
					firstError = err
				}
				continue
			}
		}
		params, err := g.GenerateParams(&requestedGenerator, appSet)
		if err != nil {
			log.WithError(err).WithField("generator", g).
				Error("error generating params")
			if firstError == nil {
				firstError = err
			}
			continue
		}

		res = append(res, TransformResult{
			Params:   params,
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

		if !reflect.ValueOf(field.Interface()).IsNil() {
			res = append(res, generators[v.Type().Field(i).Name])
		}
	}

	return res
}

func mergeGeneratorTemplate(g Generator, requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetTemplate argoprojiov1alpha1.ApplicationSetTemplate) (argoprojiov1alpha1.ApplicationSetTemplate, error) {

	// Make a copy of the value from `GetTemplate()` before merge, rather than copying directly into
	// the provided parameter (which will touch the original resource object returned by client-go)
	dest := g.GetTemplate(requestedGenerator).DeepCopy()

	err := mergo.Merge(dest, applicationSetTemplate)

	return *dest, err
}

func interpolateGenerator(requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, params []map[string]string) error {
	tmplBytes, err := json.Marshal(requestedGenerator)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error marshalling requested generator for interpolation")
		return err
	}
	tmpParams := make(map[string]string)
	for _, currParam := range params {
		for k, v := range currParam {
			tmpParams[k] = v
		}
	}

	fstTmpl := fasttemplate.New(string(tmplBytes), "{{", "}}")
	render := utils.Render{}
	replacedTmplStr, err := render.Replace(fstTmpl, tmpParams, true)
	if err != nil {
		log.WithError(err).WithField("interpolatedGeneratorString", replacedTmplStr).Error("error interpolating generator with other generator's parameter")
		return err
	}
	err = json.Unmarshal([]byte(replacedTmplStr), requestedGenerator)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error unmarshalling requested generator for interpolation")
		return err
	}
	return nil
}
