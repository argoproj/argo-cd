package kube

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func convertToVersionWithScheme(obj *unstructured.Unstructured, group string, version string) (*unstructured.Unstructured, error) {
	s := scheme.Scheme
	object, err := s.ConvertToVersion(obj, runtime.InternalGroupVersioner)
	if err != nil {
		return nil, err
	}
	unmarshalledObj, err := s.ConvertToVersion(object, schema.GroupVersion{Group: group, Version: version})
	if err != nil {
		return nil, err
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstrBody}, nil
}
