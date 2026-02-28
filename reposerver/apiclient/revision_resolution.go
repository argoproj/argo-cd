package apiclient

// RevisionResolution captures the intermediate result of resolving a revision constraint
// (e.g. semver range "v1.*") to a concrete reference (e.g. tag "v1.2.3") before final
// SHA/digest resolution. Only populated when a constraint was actually resolved; nil for
// branches, exact tags, and concrete SHAs.
//
// NOTE: This file is a temporary pre-codegen stub. After running `make protogen-fast` the
// generated repository.pb.go will contain the proper protobuf-backed version of this struct
// and this file should be removed.
type RevisionResolution struct {
	// ResolvedSymbol is the intermediate resolved reference (e.g. tag "v1.2.3") resolved
	// from a constraint (e.g. "v1.*") before the final SHA/digest resolution.
	ResolvedSymbol string `json:"resolvedSymbol,omitempty"`
	// Constraint is the original constraint expression that was resolved (e.g. "v1.*").
	Constraint string `json:"constraint,omitempty"`
}
