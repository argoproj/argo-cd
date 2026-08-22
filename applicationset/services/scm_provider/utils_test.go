package scm_provider

import (
	"testing"

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

// TestFilterCombo is the regression test for
// https://github.com/argoproj/argo-cd/issues/23881. A repo-level filter
// (RepositoryMatch) and a branch-level filter (PathsExist) in separate entries
// must be combined with OR, not implicitly AND'd across the two filtering
// stages. Note the mock's RepoHasPath returns true only when the requested path
// equals the repository name, so "two" is the only repo that "has path" "two".
func TestFilterCombo(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one"},
			{Repository: "two"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{RepositoryMatch: new("one")},
		{PathsExist: []string{"two"}},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	// "one" matches the first filter, "two" matches the second. Before the fix
	// this returned zero repos.
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
}

// TestSingleFilterRepoAndPath verifies that conditions within a single filter
// entry are AND'd together. Only "two" both matches /two/ and has path "two".
func TestSingleFilterRepoAndPath(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one"},
			{Repository: "two"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("two"),
			PathsExist:      []string{"two"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

// TestSingleFilterRepoAndPathNoMatch verifies the AND semantics reject a repo
// that satisfies only one condition of a mixed entry: "one" matches /one/ but
// does not have path "two".
func TestSingleFilterRepoAndPathNoMatch(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one"},
			{Repository: "two"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("one"),
			PathsExist:      []string{"two"},
		},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Empty(t, repos)
}

// TestMixedFilterWithRepoFilter guards the misclassification bug: a mixed entry
// (repo-level AND branch-level conditions) combined with a pure repo-level entry
// must not cause the mixed entry to be dropped from repo-stage consideration.
func TestMixedFilterWithRepoFilter(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one"},
			{Repository: "two"},
			{Repository: "three"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: new("one"),
			PathsExist:      []string{"one"}, // "one" matches /one/ and has path "one"
		},
		{RepositoryMatch: new("three")}, // pure repo-level filter
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "three", repos[1].Repository)
}

// TestFilterPathsDoNotExistCombo verifies OR between a repo-level filter and a
// PathsDoNotExist branch-level filter. "two" is the only repo that has path
// "two", so PathsDoNotExist:["two"] matches "one" and "three".
func TestFilterPathsDoNotExistCombo(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one"},
			{Repository: "two"},
			{Repository: "three"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{RepositoryMatch: new("one")},
		{PathsDoNotExist: []string{"two"}},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "three", repos[1].Repository)
}

// TestFilterComboRepoOrBranch verifies OR across a repo-level filter and a
// branch-level filter when repos expand into multiple branches. Expect every
// branch of repo "two" plus any branch named "feature".
func TestFilterComboRepoOrBranch(t *testing.T) {
	t.Parallel()
	provider := &MockProvider{
		Repos: []*Repository{
			{Repository: "one", Branch: "main"},
			{Repository: "one", Branch: "feature"},
			{Repository: "two", Branch: "main"},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{RepositoryMatch: new("two")},
		{BranchMatch: new("feature")},
	}
	repos, err := ListRepos(t.Context(), provider, filters, "")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "feature", repos[0].Branch)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "main", repos[1].Branch)
}
