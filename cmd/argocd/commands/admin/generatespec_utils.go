package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	ioutil "github.com/argoproj/argo-cd/v3/util/io"
)

func getOutWriter(inline bool, filePath string) (io.Writer, io.Closer, error) {
	if !inline {
		return os.Stdout, ioutil.NopCloser, nil
	}

	if filePath == "" {
		return nil, nil, errors.New("The file path must be specified using flag '--file'")
	}

	err := os.Rename(filePath, filePath+".back")
	if err != nil {
		return nil, nil, err
	}

	fileOut, err := os.Create(filePath)
	if err != nil {
		return nil, nil, err
	}
	return fileOut, fileOut, nil
}

// PrintResources prints a single resource in YAML or JSON format to stdout according to the output format
func PrintResources(output string, out io.Writer, resources ...any) error {
	for i, resource := range resources {
		if secret, ok := resource.(*corev1.Secret); ok {
			convertSecretData(secret)
		}
		filteredResource, err := omitFields(resource)
		if err != nil {
			return fmt.Errorf("error omitting filtered fields from the resource: %w", err)
		}
		resources[i] = filteredResource
	}
	var obj any = resources
	if len(resources) == 1 {
		obj = resources[0]
	}

	switch output {
	case "json":
		jsonBytes, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling json: %w", err)
		}

		_, _ = fmt.Fprintln(out, string(jsonBytes))
	case "yaml":
		yamlBytes, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("error marshaling yaml: %w", err)
		}
		// marshaled YAML already ends with the new line character
		_, _ = fmt.Fprint(out, string(yamlBytes))
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}

// omit fields such as status, creationTimestamp and metadata.namespace in k8s objects
func omitFields(resource any) (any, error) {
	jsonBytes, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	toMap := make(map[string]any)
	err = json.Unmarshal(jsonBytes, &toMap)
	if err != nil {
		return nil, err
	}

	delete(toMap, "status")
	if v, ok := toMap["metadata"]; ok {
		if metadata, ok := v.(map[string]any); ok {
			delete(metadata, "creationTimestamp")
			delete(metadata, "namespace")
		}
	}
	return toMap, nil
}

// convertSecretData converts kubernetes secret's data to stringData
func convertSecretData(secret *corev1.Secret) {
	secret.Kind = kube.SecretKind
	secret.APIVersion = "v1"
	secret.StringData = map[string]string{}
	for k, v := range secret.Data {
		secret.StringData[k] = string(v)
	}
	secret.Data = map[string][]byte{}
}
