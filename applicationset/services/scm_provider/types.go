package scm_provider

import (
	"context"
	"regexp"
)

// An abstract repository from an API provider.
type Repository struct {
	Organization string
	Repository   string
	URL          string
	Branch       string
	SHA          string
	Labels       []string
	RepositoryId any
}

type SCMProviderService interface {
	ListRepos(context.Context, string) ([]*Repository, error)
	RepoHasPath(context.Context, *Repository, string) (bool, error)
	GetBranches(context.Context, *Repository) ([]*Repository, error)
}

// Filter is a compiled version of SCMProviderGeneratorFilter for performance.
type Filter struct {
	RepositoryMatch *regexp.Regexp
	PathsExist      []string
	PathsDoNotExist []string
	LabelMatch      *regexp.Regexp
	BranchMatch     *regexp.Regexp
	// FilterType is only consulted by the legacy filter evaluation, which splits
	// filters into repo-level and branch-level groups. It is unused when the
	// corrected evaluation is enabled, and can be removed together with the
	// legacy path. See ListRepos.
	FilterType FilterType
}

// A convenience type for indicating where to apply a filter.
//
// Deprecated: only used by the legacy filter evaluation. A filter that mixes
// repo-level and branch-level conditions cannot be represented by a single type,
// which is the root of https://github.com/argoproj/argo-cd/issues/23881.
type FilterType int64

// The enum of filter types
const (
	FilterTypeUndefined FilterType = iota
	FilterTypeBranch
	FilterTypeRepo
)
