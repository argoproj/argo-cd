package scheme

import (
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

// ResolveParseableType will build and return a ParseableType object
// based on the given gvk and GvkParser. If the given gvkParser is nil
// it will return a DeducedParseableType which is not suitable for
// calculating diffs. Will use the statically defined schema for k8s
// built in types. Will rely on the given gvk parser for CRD schemas.
func ResolveParseableType(gvk schema.GroupVersionKind, parser *managedfields.GvkParser) *typed.ParseableType {
	if parser == nil {
		return &typed.DeducedParseableType
	}
	pt := resolveFromStaticParser(gvk, parser)
	if pt == nil {
		return parser.Type(gvk)
	}
	return pt
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
