package generators

import (
	"fmt"

	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func appendTemplatedValues(values map[string]string, params map[string]interface{}, appSet *argoappsetv1alpha1.ApplicationSet) error {
	// We create a local map to ensure that we do not fall victim to a billion-laughs attack. We iterate through the
	// cluster values map and only replace values in said map if it has already been allowlisted in the params map.
	// Once we iterate through all the cluster values we can then safely merge the `tmp` map into the main params map.
	tmp := map[string]interface{}{}

	for key, value := range values {
		result, err := replaceTemplatedString(value, params, appSet)

		if err != nil {
			return fmt.Errorf("failed to replace templated string: %w", err)
		}

		if appSet.Spec.GoTemplate {
			if tmp["values"] == nil {
				tmp["values"] = map[string]string{}
			}
			tmp["values"].(map[string]string)[key] = result
		} else {
			tmp[fmt.Sprintf("values.%s", key)] = result
		}
	}

	for key, value := range tmp {
		params[key] = value
	}

	return nil
}

func replaceTemplatedString(value string, params map[string]interface{}, appSet *argoappsetv1alpha1.ApplicationSet) (string, error) {
	replacedTmplStr, err := render.Replace(value, params, appSet.Spec.GoTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to replace templated string with rendered values: %w", err)
	}
	return replacedTmplStr, nil
}
