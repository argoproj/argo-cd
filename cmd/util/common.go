package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

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

func SaveToFile(resource interface{}, outputFormat string, outputPath string) error {
	var data []byte
	var err error
	switch outputFormat {
	case "yaml":
		if data, err = yaml.Marshal(resource); err != nil {
			return err
		}
	case "json":
		if data, err = json.Marshal(resource); err != nil {
			return err
		}
	default:
		return fmt.Errorf("format %s is not supported", outputFormat)
	}

	return ioutil.WriteFile(outputPath, data, 0644)
}
