// Package kubemeta extracts the identifying fields (apiVersion, kind,
// metadata.name, metadata.namespace) from a Kubernetes object's JSON without
// fully unmarshalling the whole document into an unstructured.Unstructured.
//
// It decodes into a four-field struct with encoding/json/v2, which tokenizes
// and skips every other member without materializing it. Reading four fields
// out of a 64KB manifest costs one allocation instead of the ~10k that
// unmarshalling it whole would.
//
// This is the lightweight extraction used by hot paths that only need a
// resource's GroupVersionKind/name/namespace — e.g. sync's groupDiffResults,
// and Argo CD's getResourceTree / GetManifests. Anything that needs the rest of
// the object still unmarshals it properly; see the Secret branch in
// GetManifests, which parses twice on purpose.
package kubemeta
