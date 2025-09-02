package versions

// RevisionResolutionType represents the type of revision resolution that occurred
type RevisionResolutionType string

const (
	// The revision was resolved directly (exact match)
	RevisionResolutionDirect RevisionResolutionType = "direct"
	// The revision was resolved from a semver constraint/range
	RevisionResolutionRange RevisionResolutionType = "range"
	// The revision was resolved from a symbolic reference (e.g., HEAD)
	RevisionResolutionSymbolicReference RevisionResolutionType = "symbolic_reference"
	// The revision was assumed to be a truncated commit SHA
	RevisionResolutionTruncatedCommitSHA RevisionResolutionType = "truncated_commit_sha"
	// The revision was resolved as a specific version, e.g. "v1.0.0"
	RevisionResolutionVersion RevisionResolutionType = "version"
)

// RevisionMetadata contains metadata about how a revision was resolved
type RevisionMetadata struct {
	// OriginalRevision is the original revision string provided by the user
	OriginalRevision string
	// ResolutionType indicates how the revision was resolved
	ResolutionType RevisionResolutionType
	// ResolvedTag is the actual tag/version that was resolved (if applicable)
	ResolvedTag string
	// ResolvedTo is the target of a symbolic reference resolution (Git-specific)
	ResolvedTo string
}

// IsEmpty returns true if the metadata has no meaningful data
func (m *RevisionMetadata) IsEmpty() bool {
	return m == nil || (m.OriginalRevision == "" && m.ResolutionType == "" && m.ResolvedTag == "" && m.ResolvedTo == "")
}

// NewRevisionMetadata creates a new RevisionMetadata with the given parameters
func NewRevisionMetadata(originalRevision string, resolutionType RevisionResolutionType) *RevisionMetadata {
	return &RevisionMetadata{
		OriginalRevision: originalRevision,
		ResolutionType:   resolutionType,
	}
}

// WithResolvedTag sets the resolved tag and returns the metadata for chaining
func (m *RevisionMetadata) WithResolvedTag(tag string) *RevisionMetadata {
	m.ResolvedTag = tag
	return m
}

// WithResolvedTo sets the resolved target and returns the metadata for chaining
func (m *RevisionMetadata) WithResolvedTo(target string) *RevisionMetadata {
	m.ResolvedTo = target
	return m
}
