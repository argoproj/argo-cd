package scheme

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// GVKParser resolves a GroupVersionKind to its structured-merge-diff
// ParseableType. Implementations may load schemas eagerly (e.g. from
// an OpenAPI v2 document) or lazily (e.g. per-GroupVersion from v3).
//
// Type returns the ParseableType for the given GVK, or an error if the
// schema could not be loaded (e.g. bad CRD, network failure). A nil
// ParseableType with a nil error means the GVK is simply not known.
type GVKParser interface {
	Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error)
}

// GVKErrorReporter is optionally implemented by GVKParser implementations
// that support injecting per-GVK errors from external sources. For example,
// the cluster cache reports list/watch failures (e.g. conversion webhook
// down) so that the error surfaces through Type() to consumers.
type GVKErrorReporter interface {
	ReportError(gvk schema.GroupVersionKind, err error)
}
