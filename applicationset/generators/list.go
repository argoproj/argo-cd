package generators

import (
	"encoding/json"
	"fmt"
	"time"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*ListGenerator)(nil)

type ListGenerator struct {
}

func NewListGenerator() Generator {
	g := &ListGenerator{}
	return g
}

func (g *ListGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ListGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.List.Template
}

func (g *ListGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.List == nil {
		return nil, EmptyAppSetGeneratorError
	}

	res := make([]map[string]interface{}, len(appSetGenerator.List.Elements))

	for i, tmpItem := range appSetGenerator.List.Elements {
		params := map[string]interface{}{}
		var element map[string]interface{}
		err := json.Unmarshal(tmpItem.Raw, &element)
		if err != nil {
			return nil, fmt.Errorf("error unmarshling list element %v", err)
		}

		if appSet.Spec.GoTemplate {
			res[i] = element
		} else {
			for key, value := range element {
				if key == "values" {
					values, ok := (value).(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("error parsing values map")
					}
					for k, v := range values {
						value, ok := v.(string)
						if !ok {
							return nil, fmt.Errorf("error parsing value as string %v", err)
						}
						params[fmt.Sprintf("values.%s", k)] = value
					}
				} else {
					v, ok := value.(string)
					if !ok {
						return nil, fmt.Errorf("error parsing value as string %v", err)
					}
					params[key] = v
				}
				res[i] = params
			}
		}
	}

	return res, nil
}
