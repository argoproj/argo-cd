package scheme

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// ResolveParseableType will build and return a ParseableType object
// based on the given gvk and GVKParser. If the given parser is nil
// it will return a DeducedParseableType which is not suitable for
// calculating diffs. Will use the statically defined schema for k8s
// built in types. Will rely on the given parser for CRD schemas.
//
// Returns an error if the parser fails to load the schema for the GVK
// (e.g. bad CRD, network failure). A nil ParseableType with nil error
// means the GVK is not known.
func ResolveParseableType(gvk schema.GroupVersionKind, parser GVKParser) (*typed.ParseableType, error) {
	if parser == nil {
		return &typed.DeducedParseableType, nil
	}
	pt := resolveFromStaticParser(gvk)
	if pt == nil {
		return parser.Type(gvk)
	}
	return pt, nil
}

func resolveFromStaticParser(gvk schema.GroupVersionKind) *typed.ParseableType {
	name := resolveModelName(gvk)
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

// resolveModelName returns the OpenAPI model name for a GVK by deriving
// it from the k8s type registry. This allows the static parser optimization
// to work with any GVKParser implementation.
func resolveModelName(gvk schema.GroupVersionKind) string {
	return getSchemeModelName(gvk)
}

var (
	schemeModelNames     map[schema.GroupVersionKind]string
	schemeModelNamesOnce sync.Once
)

// getSchemeModelName derives the OpenAPI model name for a GVK from the
// k8s type registry. This allows the static parser optimization to work
// with any GVKParser implementation (e.g. the lazy v3 parser) without
// requiring reflection on a concrete *managedfields.GvkParser.
func getSchemeModelName(gvk schema.GroupVersionKind) string {
	schemeModelNamesOnce.Do(func() {
		schemeModelNames = buildSchemeModelNames()
	})
	return schemeModelNames[gvk]
}

// buildSchemeModelNames builds a GVK→model name map from all external types
// registered in the k8s scheme. The model name follows the OpenAPI convention:
// k8s.io/api/apps/v1.Deployment → io.k8s.api.apps.v1.Deployment
func buildSchemeModelNames() map[schema.GroupVersionKind]string {
	results := make(map[schema.GroupVersionKind]string)
	for gvk, t := range Scheme.AllKnownTypes() {
		pkgPath := t.PkgPath()
		if !strings.HasPrefix(pkgPath, "k8s.io/api/") {
			continue
		}
		// Convert Go package path to OpenAPI model name:
		// k8s.io/api/apps/v1 + Deployment → io.k8s.api.apps.v1.Deployment
		modelName := strings.Replace(pkgPath, "k8s.io/", "io.k8s.", 1)
		modelName = strings.ReplaceAll(modelName, "/", ".")
		modelName += "." + t.Name()
		results[gvk] = modelName
	}
	return results
}
