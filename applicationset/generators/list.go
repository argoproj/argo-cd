package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*ListGenerator)(nil)

type ListGenerator struct{}

func NewListGenerator() Generator {
	g := &ListGenerator{}
	return g
}

func (g *ListGenerator) GetRequeueAfter(_ *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ListGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.List.Template
}

func (g *ListGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.List == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	res := make([]map[string]any, 0, len(appSetGenerator.List.Elements))

	for _, tmpItem := range appSetGenerator.List.Elements {
		var element map[string]any
		err := json.Unmarshal(tmpItem.Raw, &element)
		if err != nil {
			return nil, fmt.Errorf("error unmarshling list element %w", err)
		}

		processed, err := processElement(element, appSet.Spec.GoTemplate)
		if err != nil {
			return nil, err
		}
		res = append(res, processed)
	}

	// Append elements from ElementsYaml to the response
	if appSetGenerator.List.ElementsYaml != "" {
		var yamlElements []map[string]any
		err := yaml.Unmarshal([]byte(appSetGenerator.List.ElementsYaml), &yamlElements)
		if err != nil {
			return nil, fmt.Errorf("error unmarshling decoded ElementsYaml %w", err)
		}
		for _, element := range yamlElements {
			processed, err := processElement(element, appSet.Spec.GoTemplate)
			if err != nil {
				return nil, err
			}
			res = append(res, processed)
		}
	}

	return res, nil
}

func processElement(element map[string]any, goTemplate bool) (map[string]any, error) {
	if goTemplate {
		return element, nil
	}
	params := make(map[string]any)
	for key, value := range element {
		if key == "values" {
			values, ok := (value).(map[string]any)
			if !ok {
				return nil, errors.New("error parsing values map")
			}
			for k, v := range values {
				val, err := getParamValue(v, k)
				if err != nil {
					return nil, err
				}
				params["values."+k] = val
			}
		} else {
			val, err := getParamValue(value, key)
			if err != nil {
				return nil, err
			}
			params[key] = val
		}
	}
	return params, nil
}

func getParamValue(value any, key string) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case bool, float64:
		return fmt.Sprintf("%v", v), nil
	// JSON unmarshals to float64, but we keep int/int64 for safety also for use in manual maps/tests
	case int, int64:
		return fmt.Sprintf("%v", v), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported value type %T for key %q: nested objects and arrays are not supported in non-GoTemplate mode", value, key)
	}
}
