package generators

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

const (
	selectorKey = "Selector"
)

type TransformResult struct {
	Params   []map[string]string
	Template argoprojiov1alpha1.ApplicationSetTemplate
}

//Transform a spec generator to list of paramSets and a template
func Transform(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, allGenerators map[string]Generator, baseTemplate argoprojiov1alpha1.ApplicationSetTemplate, appSet *argoprojiov1alpha1.ApplicationSet) ([]TransformResult, error) {
	selector, err := metav1.LabelSelectorAsSelector(requestedGenerator.Selector)
	if err != nil {
		return nil, err
	}

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

		params, err := g.GenerateParams(&requestedGenerator, appSet)
		if err != nil {
			log.WithError(err).WithField("generator", g).
				Error("error generating params")
			if firstError == nil {
				firstError = err
			}
			continue
		}
		var filterParams []map[string]string
		for _, param := range params {
			if requestedGenerator.Selector != nil && !selector.Matches(labels.Set(param)) {
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

func mergeGeneratorTemplate(g Generator, requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetTemplate argoprojiov1alpha1.ApplicationSetTemplate) (argoprojiov1alpha1.ApplicationSetTemplate, error) {

	// Make a copy of the value from `GetTemplate()` before merge, rather than copying directly into
	// the provided parameter (which will touch the original resource object returned by client-go)
	dest := g.GetTemplate(requestedGenerator).DeepCopy()

	err := mergo.Merge(dest, applicationSetTemplate)

	return *dest, err
}
