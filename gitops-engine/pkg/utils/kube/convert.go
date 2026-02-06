package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"
)

func convertToVersionWithScheme(obj *unstructured.Unstructured, group string, version string) (*unstructured.Unstructured, error) {
	s := scheme.Scheme
	object, err := s.ConvertToVersion(obj, runtime.InternalGroupVersioner)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to version using internal group versioner: %w", err)
	}
	unmarshalledObj, err := s.ConvertToVersion(object, schema.GroupVersion{Group: group, Version: version})
	if err != nil {
		return nil, fmt.Errorf("failed to convert to version: %w", err)
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured object: %w", err)
	}
	return &unstructured.Unstructured{Object: unstrBody}, nil
}
