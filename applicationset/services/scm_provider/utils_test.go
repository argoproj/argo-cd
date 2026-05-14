package scm_provider

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

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
			RepositoryMatch: new("n|hr"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
			LabelMatch: new("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
			RepositoryMatch: new("("),
		},
	}
	_, err := ListRepos(t.Context(), provider, filters, "", false)
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
			LabelMatch: new("("),
		},
	}
	_, err := ListRepos(t.Context(), provider, filters, "", false)
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
			BranchMatch: new("w"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
			RepositoryMatch: new("w"),
			LabelMatch:      new("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

// TestMixedFilterRepoAndPathsExist verifies that when enableCrossStageFiltering is true,
// a filter with both repositoryMatch (repo-level) and pathsExist (branch-level) conditions
// correctly ANDs them together.
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
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("^eks"),
			PathsExist:      []string{"eks-app-one"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app-one", repos[0].Repository)
}

// TestMixedFilterRepoAndPathsExistLegacy verifies that without enableCrossStageFiltering,
// the legacy behavior is preserved where mixed filters are classified by FilterType and
// the repositoryMatch is ignored (filter is classified as branch-only).
func TestMixedFilterRepoAndPathsExistLegacy(t *testing.T) {
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
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("^eks"),
			PathsExist:      []string{"eks-app-one"},
		},
	}
	// Legacy behavior: the filter gets FilterType=FilterTypeBranch (pathsExist overwrites repositoryMatch's FilterType),
	// so repositoryMatch is effectively ignored during branch filtering, and all repos with the path match.
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
	require.NoError(t, err)
	// In legacy mode, repositoryMatch is still checked in matchFilterLegacy, so only eks repos
	// with the path match. But the filter is only in the branch group, so all repos pass the
	// repo phase (no repo filters), then in the branch phase matchFilterLegacy checks ALL conditions.
	// So: eks-app-one matches (^eks AND has path "eks-app-one"), other-app doesn't match ^eks.
	// eks-app-two matches ^eks but doesn't have path. Result: only eks-app-one.
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app-one", repos[0].Repository)
}

// TestMixedFilterRepoAndPathsDoNotExist verifies that repositoryMatch and pathsDoNotExist are ANDed
// when enableCrossStageFiltering is true.
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
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("^eks"),
			PathsDoNotExist: []string{"eks-app-one"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "eks-app-two", repos[0].Repository)
}

// TestMixedFilterRepoAndBranchMatch verifies that repositoryMatch and branchMatch are ANDed
// when enableCrossStageFiltering is true.
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
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("^eks"),
			BranchMatch:     new("^main$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
			RepositoryMatch: new("e"),
		},
		{
			LabelMatch: new("^prod-.*$"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", false)
	require.NoError(t, err)
	assert.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "three", repos[2].Repository)
}

// tests the getApplicableFilters function with legacy FilterType-based categorization
func TestApplicableFilterMapLegacy(t *testing.T) {
	branchFilter := Filter{
		BranchMatch: &regexp.Regexp{},
		FilterType:  FilterTypeBranch,
	}
	repoFilter := Filter{
		RepositoryMatch: &regexp.Regexp{},
		FilterType:      FilterTypeRepo,
	}
	pathExistsFilter := Filter{
		PathsExist: []string{"test"},
		FilterType: FilterTypeBranch,
	}
	pathDoesntExistsFilter := Filter{
		PathsDoNotExist: []string{"test"},
		FilterType:      FilterTypeBranch,
	}
	labelMatchFilter := Filter{
		LabelMatch: &regexp.Regexp{},
		FilterType: FilterTypeRepo,
	}
	unsetFilter := Filter{
		LabelMatch: &regexp.Regexp{},
	}
	additionalBranchFilter := Filter{
		BranchMatch: &regexp.Regexp{},
		FilterType:  FilterTypeBranch,
	}
	filterMap := getApplicableFilters([]*Filter{
		&branchFilter, &repoFilter,
		&pathExistsFilter, &labelMatchFilter, &unsetFilter, &additionalBranchFilter, &pathDoesntExistsFilter,
	}, false)

	assert.Len(t, filterMap[FilterTypeRepo], 2)
	assert.Len(t, filterMap[FilterTypeBranch], 4)
}

// tests the getApplicableFilters function categorizes filters based on their actual conditions
// when enableCrossStage is true
func TestApplicableFilterMapCrossStage(t *testing.T) {
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
	}, true)

	// repoFilter and labelMatchFilter have repo-level conditions
	assert.Len(t, filterMap[FilterTypeRepo], 2)
	// branchFilter, pathExistsFilter, additionalBranchFilter, pathDoesntExistsFilter have branch-level conditions
	assert.Len(t, filterMap[FilterTypeBranch], 4)
}

// tests that a filter with both repo and branch conditions is added to both filter groups
// when enableCrossStage is true
func TestApplicableFilterMapMixedFilter(t *testing.T) {
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
	}, true)

	// mixedFilter and repoOnlyFilter have repo-level conditions
	assert.Len(t, filterMap[FilterTypeRepo], 2)
	assert.Contains(t, filterMap[FilterTypeRepo], &mixedFilter)
	assert.Contains(t, filterMap[FilterTypeRepo], &repoOnlyFilter)

	// mixedFilter and branchOnlyFilter have branch-level conditions
	assert.Len(t, filterMap[FilterTypeBranch], 2)
	assert.Contains(t, filterMap[FilterTypeBranch], &mixedFilter)
	assert.Contains(t, filterMap[FilterTypeBranch], &branchOnlyFilter)
}
