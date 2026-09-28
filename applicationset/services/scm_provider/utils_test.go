package scm_provider

import (
	"strings"
	"testing"

	"github.com/dlclark/regexp2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestFilterRepoMatch(t *testing.T) {
	t.Parallel()
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
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "three", repos[1].Repository)
}

func TestFilterLabelMatch(t *testing.T) {
	t.Parallel()
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
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
}

func TestFilterPathExists(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	_, err := ListRepos(t.Context(), provider, filters, "")
	require.Error(t, err)
}

func TestFilterLabelMatchBadRegexp(t *testing.T) {
	t.Parallel()
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
	_, err := ListRepos(t.Context(), provider, filters, "")
	require.Error(t, err)
}

func TestFilterBranchMatch(t *testing.T) {
	t.Parallel()
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
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[0].Branch)
	assert.Equal(t, "three", repos[1].Repository)
	assert.Equal(t, "two", repos[1].Branch)
}

func TestFilterBranchMatchLookahead(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Branch:     "feature/one",
			},
			{
				Repository: "two",
				Branch:     "release/1.0",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			BranchMatch: new("^(?!release/).*"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "feature/one", repos[0].Branch)
}

func TestFilterLabelMatchLookbehind(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"env: prod"},
			},
			{
				Repository: "two",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: new("(?<=env: ).*"),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "one", repos[0].Repository)
}

// Patterns written for Go's regexp package must keep their meaning: \d only matches ASCII digits.
func TestFilterRepoMatchKeepsGoRegexpSemantics(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "repo-123",
			},
			{
				Repository: "repo-1١٢",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new(`^repo-1\d{2}$`),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "repo-123", repos[0].Repository)
}

func TestFilterLabelMatchNamedGroup(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"env-prod"},
			},
			{
				Repository: "two",
				Labels:     []string{"team-a"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: new(`^env-(?P<name>prod|dev)$`),
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "one", repos[0].Repository)
}

// A filter that cannot be evaluated must fail generation. Silently dropping the repository would make the
// generator succeed with fewer results, and the ApplicationSet controller would delete the repository's Application.
func TestFilterMatchErrorFailsGeneration(t *testing.T) {
	t.Parallel()
	evilInput := strings.Repeat("a", 28) + "!"
	tests := []struct {
		name   string
		filter argoprojiov1alpha1.SCMProviderGeneratorFilter
	}{
		{"RepositoryMatch", argoprojiov1alpha1.SCMProviderGeneratorFilter{RepositoryMatch: new(`^(a+)+$`)}},
		{"LabelMatch", argoprojiov1alpha1.SCMProviderGeneratorFilter{LabelMatch: new(`^(a+)+$`)}},
		{"BranchMatch", argoprojiov1alpha1.SCMProviderGeneratorFilter{BranchMatch: new(`^(a+)+$`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := &MockProvider{
				Repos: []*Repository{
					{
						Repository: evilInput,
						Branch:     evilInput,
						Labels:     []string{evilInput},
					},
				},
			}
			repos, err := ListRepos(t.Context(), provider, []argoprojiov1alpha1.SCMProviderGeneratorFilter{tt.filter}, "")
			require.Error(t, err)
			assert.Nil(t, repos)
		})
	}
}

func TestMultiFilterAnd(t *testing.T) {
	t.Parallel()
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
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

func TestMultiFilterOr(t *testing.T) {
	t.Parallel()
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
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "three", repos[2].Repository)
}

func TestNoFilters(t *testing.T) {
	t.Parallel()
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

// tests the getApplicableFilters function, passing in all the filters, and an unset filter, plus an additional
// branch filter
func TestApplicableFilterMap(t *testing.T) {
	t.Parallel()
	branchFilter := Filter{
		BranchMatch: &regexp2.Regexp{},
		FilterType:  FilterTypeBranch,
	}
	repoFilter := Filter{
		RepositoryMatch: &regexp2.Regexp{},
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
		LabelMatch: &regexp2.Regexp{},
		FilterType: FilterTypeRepo,
	}
	unsetFilter := Filter{
		LabelMatch: &regexp2.Regexp{},
	}
	additionalBranchFilter := Filter{
		BranchMatch: &regexp2.Regexp{},
		FilterType:  FilterTypeBranch,
	}
	filterMap := getApplicableFilters([]*Filter{
		&branchFilter, &repoFilter,
		&pathExistsFilter, &labelMatchFilter, &unsetFilter, &additionalBranchFilter, &pathDoesntExistsFilter,
	})

	assert.Len(t, filterMap[FilterTypeRepo], 2)
	assert.Len(t, filterMap[FilterTypeBranch], 4)
}
