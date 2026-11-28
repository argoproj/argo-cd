package fieldmanager

/*
In order to keep maintenance as minimal as possible the borrowed
files in this package are verbatim copy from Kubernetes. The
private objects that need to be exposed are wrapped and exposed
in this file.
*/

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/merge"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/scheme"
)

// NewVersionConverter will expose the version converter from the
// borrowed private function from k8s apiserver handler.
func NewVersionConverter(resolver scheme.TypeResolver, o runtime.ObjectConvertor, h schema.GroupVersion) merge.Converter {
	tc := &resolverTypeConverter{resolver: resolver}
	hub := schema.GroupVersion{Group: h.Group, Version: runtime.APIVersionInternal}
	return newVersionConverter(tc, o, hub)
}

// resolverTypeConverter is the borrowed typeConverter with types looked up through a
// scheme.TypeResolver instead of a managedfields.GvkParser.
type resolverTypeConverter struct {
	resolver scheme.TypeResolver
}

var _ TypeConverter = &resolverTypeConverter{}

func (c *resolverTypeConverter) ObjectToTyped(obj runtime.Object) (*typed.TypedValue, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	t, err := c.resolver.Type(gvk)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, newNoCorrespondingTypeError(gvk)
	}
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		return t.FromUnstructured(o.UnstructuredContent())
	default:
		return t.FromStructured(obj)
	}
}

func (c *resolverTypeConverter) TypedToObject(value *typed.TypedValue) (runtime.Object, error) {
	return valueToObject(value.AsValue())
}
