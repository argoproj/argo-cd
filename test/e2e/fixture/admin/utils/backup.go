package utils

import (
	"fmt"
	"strings"

	kube "github.com/argoproj/gitops-engine/pkg/utils/kube"
	yaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ExportedResources []unstructured.Unstructured

func GetExportedResourcesFromOutput(output string) (ExportedResources, error) {
	var resources []unstructured.Unstructured
	docs := strings.Split(output, "---")

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		var resourceData map[string]interface{}

		if err := yaml.Unmarshal([]byte(doc), &resourceData); err != nil {
			return nil, fmt.Errorf("error unmarshaling YAML: %w", err)
		}

		resource := unstructured.Unstructured{Object: resourceData}
		resources = append(resources, resource)
	}

	return resources, nil
}

func (e ExportedResources) HasResource(resource kube.ResourceKey) bool {
	for _, res := range e {
		if res.GetObjectKind().GroupVersionKind().Group == resource.Group &&
			res.GetKind() == resource.Kind &&
			res.GetName() == resource.Name &&
			res.GetNamespace() == resource.Namespace {
			return true
		}
	}

	return false
}
