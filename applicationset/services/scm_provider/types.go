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
	RepositoryId interface{}
}

type SCMProviderService interface {
	ListRepos(context.Context, string) ([]*Repository, error)
	RepoHasPath(context.Context, *Repository, string) (bool, error)
	GetBranches(context.Context, *Repository) ([]*Repository, error)
}

// A compiled version of SCMProviderGeneratorFilter for performance.
type Filter struct {
	RepositoryMatch *regexp.Regexp
	PathsExist      []string
	PathsDoNotExist []string
	LabelMatch      *regexp.Regexp
	BranchMatch     *regexp.Regexp
	FilterType      FilterType
}

// A convenience type for indicating where to apply a filter
type FilterType int64

// The enum of filter types
const (
	FilterTypeUndefined FilterType = iota
	FilterTypeBranch
	FilterTypeRepo
)
