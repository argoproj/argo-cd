package scm_provider

import (
	"regexp"
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	_, err := ListRepos(t.Context(), provider, filters, "", true)
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
	_, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
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
	repos, err := ListRepos(t.Context(), provider, filters, "", true)
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "feature", repos[0].Branch)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "main", repos[1].Branch)
}

// TestLegacyFilterEvaluation pins the pre-fix behaviour that is still the
// default, so that flipping `--enable-new-scm-provider-filtering` is a
// deliberate, reviewable change rather than an accident. Every case below is a
// combination the corrected evaluation reports differently; the paired
// expectations are what make the flag's effect explicit.
//
// Remove this test together with listReposLegacyFiltering.
func TestLegacyFilterEvaluation(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// repos is the mock's repo/branch inventory.
		repos []*Repository
		// filters is the ApplicationSet's filter list.
		filters []argoprojiov1alpha1.SCMProviderGeneratorFilter
		// legacy is what the default evaluation returns, as "repo/branch".
		legacy []string
		// fixed is what --enable-new-scm-provider-filtering returns.
		fixed []string
	}{
		{
			// https://github.com/argoproj/argo-cd/issues/23881: a repo-level and
			// a branch-level filter are AND'd across stages, so nothing survives.
			name:    "repo-level OR branch-level yields nothing",
			repos:   []*Repository{{Repository: "one"}, {Repository: "two"}},
			filters: []argoprojiov1alpha1.SCMProviderGeneratorFilter{{RepositoryMatch: new("one")}, {PathsExist: []string{"two"}}},
			legacy:  []string{},
			fixed:   []string{"one/", "two/"},
		},
		{
			// A filter mixing repo-level and branch-level conditions is
			// classified by whichever condition was assigned last, so the mixed
			// filter is never considered at the repo stage.
			name:  "mixed filter alongside a repo-level filter yields nothing",
			repos: []*Repository{{Repository: "one"}, {Repository: "two"}, {Repository: "three"}},
			filters: []argoprojiov1alpha1.SCMProviderGeneratorFilter{
				{RepositoryMatch: new("one"), PathsExist: []string{"one"}},
				{RepositoryMatch: new("three")},
			},
			legacy: []string{},
			fixed:  []string{"one/", "three/"},
		},
		{
			// Here the legacy path returns a strict subset: "three" satisfies the
			// pathsDoNotExist filter but never reaches the branch stage, because
			// it failed the unrelated repositoryMatch filter first.
			name:    "pathsDoNotExist OR repo-level drops repos that satisfy only the branch filter",
			repos:   []*Repository{{Repository: "one"}, {Repository: "two"}, {Repository: "three"}},
			filters: []argoprojiov1alpha1.SCMProviderGeneratorFilter{{RepositoryMatch: new("one")}, {PathsDoNotExist: []string{"two"}}},
			legacy:  []string{"one/"},
			fixed:   []string{"one/", "three/"},
		},
		{
			// Note that even "two/main", which fully satisfies the repo-level
			// filter on its own, is dropped: it still has to clear some
			// branch-level filter, and branchMatch=feature rejects it.
			name: "repositoryMatch OR branchMatch yields nothing",
			repos: []*Repository{
				{Repository: "one", Branch: "main"},
				{Repository: "one", Branch: "feature"},
				{Repository: "two", Branch: "main"},
			},
			filters: []argoprojiov1alpha1.SCMProviderGeneratorFilter{{RepositoryMatch: new("two")}, {BranchMatch: new("feature")}},
			legacy:  []string{},
			fixed:   []string{"one/feature", "two/main"},
		},
		{
			// A single filter's conditions are AND'd correctly either way, which
			// is why single-filter ApplicationSets are unaffected by the flag.
			name:  "single mixed filter is unaffected",
			repos: []*Repository{{Repository: "one"}, {Repository: "two"}},
			filters: []argoprojiov1alpha1.SCMProviderGeneratorFilter{
				{RepositoryMatch: new("two"), PathsExist: []string{"two"}},
			},
			legacy: []string{"two/"},
			fixed:  []string{"two/"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				enableNewFiltering bool
				want               []string
			}{
				{false, c.legacy},
				{true, c.fixed},
			} {
				provider := &MockProvider{Repos: c.repos}
				repos, err := ListRepos(t.Context(), provider, c.filters, "", tc.enableNewFiltering)
				require.NoError(t, err)
				got := make([]string, 0, len(repos))
				for _, r := range repos {
					got = append(got, r.Repository+"/"+r.Branch)
				}
				assert.Equal(t, tc.want, got, "enableNewFiltering=%v", tc.enableNewFiltering)
			}
		})
	}
}

// TestApplicableFilterMap covers the filter classification used by the legacy
// evaluation: all the filter kinds, an unset filter, plus an additional branch
// filter.
//
// Remove this test together with getApplicableFilters.
func TestApplicableFilterMap(t *testing.T) {
	t.Parallel()
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
	})

	assert.Len(t, filterMap[FilterTypeRepo], 2)
	assert.Len(t, filterMap[FilterTypeBranch], 4)
}
