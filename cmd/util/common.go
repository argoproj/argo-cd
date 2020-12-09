package util

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
)

// PrintResource prints a single resource in YAML or JSON format to stdout according to the output format
func PrintResource(resource interface{}, output string) error {
	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(resource, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(resource)
		if err != nil {
			return err
		}
		fmt.Println(string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}
