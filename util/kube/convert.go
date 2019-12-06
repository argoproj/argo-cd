package kube

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/argoproj/argo-cd/util/tracing"
)

func convertToVersionWithScheme(obj *unstructured.Unstructured, group string, version string) (*unstructured.Unstructured, error) {
	span := tracing.StartSpan("convertToVersionWithScheme")
	defer span.Finish()
	s := legacyscheme.Scheme
	target := schema.GroupVersionKind{
		Group:   group,
		Version: version,
	}
	object, err := s.ConvertToVersion(obj, runtime.InternalGroupVersioner)
	if err != nil {
		return nil, err
	}
	unmarshalledObj, err := s.ConvertToVersion(object, target.GroupVersion())
	if err != nil {
		return nil, err
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstrBody}, nil
}
