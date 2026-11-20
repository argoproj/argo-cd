// Package kubemeta extracts the identifying fields (apiVersion, kind,
// metadata.name, metadata.namespace) from a Kubernetes object's JSON, decoding
// them into a four-field struct with encoding/json/v2 instead of materializing
// the whole document as an unstructured.Unstructured. Reading four fields out
// of a 64KB manifest costs one allocation rather than ~10k.
//
// Used by paths that only need a resource's identity — sync's groupDiffResults,
// Argo CD's getResourceTree and GetManifests. Anything needing the rest of the
// object still unmarshals it properly; see the Secret branch in GetManifests.
package kubemeta
