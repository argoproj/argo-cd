package util

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

var (
	LogFormat string
	LogLevel  string
)

// PrintResource prints a single resource in YAML or JSON format to stdout according to the output format
func PrintResources(resources []interface{}, output string) error {
	for i, resource := range resources {
		filteredResource, err := omitFields(resource)
		if err != nil {
			return err
		}
		resources[i] = filteredResource
	}

	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(resources, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(resources)
		if err != nil {
			return err
		}
		fmt.Println(string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}

// omit fields such as status, creationTimestamp and metadata.namespace in k8s objects
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

// ConvertSecretData converts kubernetes secret's data to stringData
func ConvertSecretData(secret *v1.Secret) {
	secret.Kind = kube.SecretKind
	secret.APIVersion = "v1"
	secret.StringData = map[string]string{}
	for k, v := range secret.Data {
		secret.StringData[k] = string(v)
	}
	secret.Data = map[string][]byte{}
}
