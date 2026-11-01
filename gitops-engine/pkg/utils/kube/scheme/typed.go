package scheme

import (
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	openapiutil "k8s.io/kube-openapi/pkg/util"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// ResolveParseableType will build and return a ParseableType object
// based on the given gvk and GvkParser. Will use the statically defined
// schema for k8s built in types. Will rely on the given gvk parser for CRD
// schemas. If the given gvkParser is nil (the cluster's OpenAPI document
// could not be turned into a parser), built in types still resolve to the
// static schema, named through the client-go scheme; anything else gets a
// DeducedParseableType, which is not suitable for calculating diffs.
func ResolveParseableType(gvk schema.GroupVersionKind, parser *managedfields.GvkParser) *typed.ParseableType {
	if parser == nil {
		if pt := resolveBuiltinFromScheme(gvk); pt != nil {
			return pt
		}
		return &typed.DeducedParseableType
	}
	pt := resolveFromStaticParser(gvk, parser)
	if pt == nil {
		return parser.Type(gvk)
	}
	return pt
}

// resolveBuiltinFromScheme finds the static schema type of a built in kind
// without a cluster parser: the client-go scheme gives the Go type, and the
// static schema names its types the way kube-openapi names definitions, from
// the Go package path and type name.
func resolveBuiltinFromScheme(gvk schema.GroupVersionKind) *typed.ParseableType {
	obj, err := clientgoscheme.Scheme.New(gvk)
	if err != nil {
		return nil
	}
	t := reflect.TypeOf(obj)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	p := StaticParser()
	if p == nil {
		return nil
	}
	pt := p.Type(openapiutil.ToRESTFriendlyName(t.PkgPath() + "." + t.Name()))
	if pt.IsValid() {
		return &pt
	}
	return nil
}

func resolveFromStaticParser(gvk schema.GroupVersionKind, parser *managedfields.GvkParser) *typed.ParseableType {
	gvkNameMap := getGvkMap(parser)
	name := gvkNameMap[gvk]
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

var (
	gvkMap      map[schema.GroupVersionKind]string
	extractOnce sync.Once
)

func getGvkMap(parser *managedfields.GvkParser) map[schema.GroupVersionKind]string {
	extractOnce.Do(func() {
		gvkMap = extractGvkMap(parser)
	})
	return gvkMap
}

func extractGvkMap(parser *managedfields.GvkParser) map[schema.GroupVersionKind]string {
	results := make(map[schema.GroupVersionKind]string)

	value := reflect.ValueOf(parser)
	gvkValue := reflect.Indirect(value).FieldByName("gvks")
	iter := gvkValue.MapRange()
	for iter.Next() {
		group := iter.Key().FieldByName("Group").String()
		version := iter.Key().FieldByName("Version").String()
		kind := iter.Key().FieldByName("Kind").String()
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		}
		name := iter.Value().String()
		results[gvk] = name
	}
	return results
}
