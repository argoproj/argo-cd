package commands

import (
	"encoding/json"
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"
)

const (
	cliName = "argocd"

	// DefaultSSOLocalPort is the localhost port to listen on for the temporary web server performing
	// the OAuth2 login flow.
	DefaultSSOLocalPort = 8085
)

// PrintResource prints a single resource in YAML or JSON format to stdout according to the output format
func PrintResource(resource interface{}, output string) error {
	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(resource, "", "  ")
		if err != nil {
			return fmt.Errorf("unable to marshal resource to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("unable to marshal resource to yaml: %w", err)
		}
		fmt.Print(string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}

// PrintResourceList marshals & prints a list of resources to stdout according to the output format
func PrintResourceList(resources interface{}, output string, single bool) error {
	kt := reflect.ValueOf(resources)
	// Sometimes, we want to marshal the first resource of a slice or array as single item
	if kt.Kind() == reflect.Slice || kt.Kind() == reflect.Array {
		if single && kt.Len() == 1 {
			return PrintResource(kt.Index(0).Interface(), output)
		}

		// If we have a zero len list, prevent printing "null"
		if kt.Len() == 0 {
			return PrintResource([]string{}, output)
		}
	}

	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(resources, "", "  ")
		if err != nil {
			return fmt.Errorf("unable to marshal resources to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(resources)
		if err != nil {
			return fmt.Errorf("unable to marshal resources to yaml: %w", err)
		}
		fmt.Print(string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}
