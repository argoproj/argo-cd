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
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v4/merge"
)

// NewVersionConverter will expose the version converter from the
// borrowed private function from k8s apiserver handler.
func NewVersionConverter(gvkParser *managedfields.GvkParser, o runtime.ObjectConvertor, h schema.GroupVersion) merge.Converter {
	tc := &typeConverter{parser: gvkParser}
	return newVersionConverter(tc, o, h)
}
