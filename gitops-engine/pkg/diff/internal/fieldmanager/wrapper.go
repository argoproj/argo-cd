package fieldmanager

/*
In order to keep maintenance as minimal as possible the borrowed
files in this package are verbatim copy from Kubernetes. The
private objects that need to be exposed are wrapped and exposed
in this file.
*/

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/merge"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// gvkParser resolves a GroupVersionKind to its ParseableType.
type gvkParser interface {
	Type(gvk schema.GroupVersionKind) *typed.ParseableType
}

// NewVersionConverter will expose the version converter from the
// borrowed private function from k8s apiserver handler.
func NewVersionConverter(parser gvkParser, o runtime.ObjectConvertor, h schema.GroupVersion) merge.Converter {
	tc := &typeConverter{parser: parser}
	return newVersionConverter(tc, o, h)
}
