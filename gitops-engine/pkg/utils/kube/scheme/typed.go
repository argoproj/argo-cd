package scheme

import (
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// TypeResolver resolves the type used to build structured merge diff values for a GVK.
type TypeResolver interface {
	// Type returns the type of gvk, or nil if there is no schema for it.
	Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error)
}

// ResolveParseableType will build and return a ParseableType object
// based on the given gvk and resolver. If the given resolver is nil
// it will return a DeducedParseableType which is not suitable for
// calculating diffs. Will use the statically defined schema for k8s
// built in types. Will rely on the given resolver for CRD schemas.
func ResolveParseableType(gvk schema.GroupVersionKind, resolver TypeResolver) (*typed.ParseableType, error) {
	if resolver == nil {
		return &typed.DeducedParseableType, nil
	}
	if pt := resolveFromStaticParser(gvk); pt != nil {
		return pt, nil
	}
	//nolint:wrapcheck // the resolver error already names the group version
	return resolver.Type(gvk)
}

func resolveFromStaticParser(gvk schema.GroupVersionKind) *typed.ParseableType {
	name := staticTypeName(gvk)
	if name == "" {
		return nil
	}
	p := StaticParser()
	if p == nil {
		return nil
	}
	pt := p.Type(name)
	if pt.IsValid() {
		return &pt
	}
	return nil
}

var staticTypeNames = sync.OnceValue(func() map[schema.GroupVersionKind]string {
	names := map[schema.GroupVersionKind]string{}
	for gvk, t := range Scheme.AllKnownTypes() {
		if gvk.Version == runtime.APIVersionInternal {
			continue
		}
		model := reflect.New(t).Interface()
		if namer, ok := model.(util.OpenAPIModelNamer); ok {
			names[gvk] = namer.OpenAPIModelName()
		} else {
			names[gvk] = util.ToRESTFriendlyName(util.GetCanonicalTypeName(model))
		}
	}
	return names
})

// staticTypeName returns the OpenAPI model name of a built-in gvk (e.g.
// io.k8s.api.apps.v1.Deployment), or "" if gvk isn't built in.
func staticTypeName(gvk schema.GroupVersionKind) string {
	return staticTypeNames()[gvk]
}
