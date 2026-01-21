package scm_provider

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func strp(s string) *string {
	return &s
}

func TestFilterRepoMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
			{
				Repository: "four",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("n|hr"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "three", repos[1].Repository)
}

func TestFilterLabelMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
}

func TestFilterPathExists(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			PathsExist: []string{"two"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

func TestFilterPathDoesntExists(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			PathsDoNotExist: []string{"two"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
}

func TestFilterRepoMatchBadRegexp(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("("),
		},
	}
	_, err := ListRepos(t.Context(), provider, filters, "")
	require.Error(t, err)
}

func TestFilterLabelMatchBadRegexp(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: strp("("),
		},
	}
	_, err := ListRepos(t.Context(), provider, filters, "")
	require.Error(t, err)
}

func TestFilterBranchMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Branch:     "one",
			},
			{
				Repository: "one",
				Branch:     "two",
			},
			{
				Repository: "two",
				Branch:     "one",
			},
			{
				Repository: "three",
				Branch:     "one",
			},
			{
				Repository: "three",
				Branch:     "two",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			BranchMatch: strp("w"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[0].Branch)
	assert.Equal(t, "three", repos[1].Repository)
	assert.Equal(t, "two", repos[1].Branch)
}

func TestMultiFilterAnd(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("w"),
			LabelMatch:      strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

// TestMixedFilterRepoAndPathsExist verifies that a filter with both repositoryMatch (repo-level)
// and pathsExist (branch-level) conditions correctly ANDs them together.
// This is the fix for the bug where repositoryMatch was ignored when combined with pathsExist.
func TestMixedFilterRepoAndPathsExist(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "eks-app-one",
				Branch:     "main",
			},
			{
				Repository: "eks-app-two",
				Branch:     "main",
			},
			{
				Repository: "other-app",
				Branch:     "main",
			},
		},
	}
	// Filter: repos matching "^eks" AND having path "eks-app-one" (MockProvider.RepoHasPath returns true if path == repo name)
	// Expected: only eks-app-one matches (matches ^eks AND has path)
	// eks-app-two matches ^eks but doesn't have path "eks-app-one"
	// other-app doesn't match ^eks
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("^eks"),
			PathsExist:      []string{"eks-app-one"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app-one", repos[0].Repository)
}

// TestMixedFilterRepoAndPathsDoNotExist verifies that repositoryMatch and pathsDoNotExist are ANDed.
func TestMixedFilterRepoAndPathsDoNotExist(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "eks-app-one",
				Branch:     "main",
			},
			{
				Repository: "eks-app-two",
				Branch:     "main",
			},
			{
				Repository: "other-app",
				Branch:     "main",
			},
		},
	}
	// Filter: repos matching "^eks" AND NOT having path "eks-app-one"
	// Expected: only eks-app-two matches
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("^eks"),
			PathsDoNotExist: []string{"eks-app-one"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app-two", repos[0].Repository)
}

// TestMixedFilterRepoAndBranchMatch verifies that repositoryMatch and branchMatch are ANDed.
func TestMixedFilterRepoAndBranchMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "eks-app",
				Branch:     "main",
			},
			{
				Repository: "eks-app",
				Branch:     "develop",
			},
			{
				Repository: "other-app",
				Branch:     "main",
			},
		},
	}
	// Filter: repos matching "^eks" AND branch matching "main"
	// Expected: only eks-app with main branch
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("^eks"),
			BranchMatch:     strp("^main$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app", repos[0].Repository)
	assert.Equal(t, "main", repos[0].Branch)
}

func TestMultiFilterOr(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("e"),
		},
		{
			LabelMatch: strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "three", repos[2].Repository)
}

func TestNoFilters(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "three", repos[2].Repository)
}

// tests the getApplicableFilters function categorizes filters based on their actual conditions
func TestApplicableFilterMap(t *testing.T) {
	branchFilter := Filter{
		BranchMatch: &regexp.Regexp{},
	}
	repoFilter := Filter{
		RepositoryMatch: &regexp.Regexp{},
	}
	pathExistsFilter := Filter{
		PathsExist: []string{"test"},
	}
	pathDoesntExistsFilter := Filter{
		PathsDoNotExist: []string{"test"},
	}
	labelMatchFilter := Filter{
		LabelMatch: &regexp.Regexp{},
	}
	additionalBranchFilter := Filter{
		BranchMatch: &regexp.Regexp{},
	}
	filterMap := getApplicableFilters([]*Filter{
		&branchFilter, &repoFilter,
		&pathExistsFilter, &labelMatchFilter, &additionalBranchFilter, &pathDoesntExistsFilter,
	})

	// repoFilter and labelMatchFilter have repo-level conditions
	assert.Len(t, filterMap[FilterTypeRepo], 2)
	// branchFilter, pathExistsFilter, additionalBranchFilter, pathDoesntExistsFilter have branch-level conditions
	assert.Len(t, filterMap[FilterTypeBranch], 4)
}

// tests that a filter with both repo and branch conditions is added to both filter groups
func TestApplicableFilterMapMixedFilter(t *testing.T) {
	// A filter with both repo-level (RepositoryMatch) and branch-level (PathsExist) conditions
	mixedFilter := Filter{
		RepositoryMatch: &regexp.Regexp{},
		PathsExist:      []string{"test"},
	}
	repoOnlyFilter := Filter{
		RepositoryMatch: &regexp.Regexp{},
	}
	branchOnlyFilter := Filter{
		BranchMatch: &regexp.Regexp{},
	}
	filterMap := getApplicableFilters([]*Filter{
		&mixedFilter, &repoOnlyFilter, &branchOnlyFilter,
	})

	// mixedFilter and repoOnlyFilter have repo-level conditions
	assert.Len(t, filterMap[FilterTypeRepo], 2)
	assert.Contains(t, filterMap[FilterTypeRepo], &mixedFilter)
	assert.Contains(t, filterMap[FilterTypeRepo], &repoOnlyFilter)

	// mixedFilter and branchOnlyFilter have branch-level conditions
	assert.Len(t, filterMap[FilterTypeBranch], 2)
	assert.Contains(t, filterMap[FilterTypeBranch], &mixedFilter)
	assert.Contains(t, filterMap[FilterTypeBranch], &branchOnlyFilter)
}
