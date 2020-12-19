package util

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
)

// PrintResource prints a single resource in YAML or JSON format to stdout according to the output format
func PrintResource(resource interface{}, output string) error {
	filteredResource, err := omitFields(resource)
	if err != nil {
		return err
	}

	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(filteredResource, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(filteredResource)
		if err != nil {
			return err
		}
		fmt.Println(string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}

func omitFields(resource interface{}) (interface{}, error) {
	jsonBytes, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	toMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(string(jsonBytes)), &toMap)
	if err != nil {
		return nil, err
	}

	delete(toMap, "status")
	if v, ok := toMap["metadata"]; ok {
		if metadata, ok := v.(map[string]interface{}); ok {
			delete(metadata, "creationTimestamp")
			delete(metadata, "namespace")
		}
	}
	return toMap, nil
}
