package scm_provider

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.SCMProviderGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.RepositoryMatch != nil {
			outFilter.RepositoryMatch, err = regexp.Compile(*filter.RepositoryMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling RepositoryMatch regexp %q: %w", *filter.RepositoryMatch, err)
			}
		}
		if filter.LabelMatch != nil {
			outFilter.LabelMatch, err = regexp.Compile(*filter.LabelMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling LabelMatch regexp %q: %w", *filter.LabelMatch, err)
			}
		}
		if filter.PathsExist != nil {
			outFilter.PathsExist = filter.PathsExist
		}
		if filter.PathsDoNotExist != nil {
			outFilter.PathsDoNotExist = filter.PathsDoNotExist
		}
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp.Compile(*filter.BranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %w", *filter.BranchMatch, err)
			}
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

// matchRepoFilter reports whether repo satisfies the repo-level conditions
// (RepositoryMatch, LabelMatch) of a single filter. A filter with no repo-level
// conditions matches any repository
func matchRepoFilter(repo *Repository, filter *Filter) bool {
	// If repositoryMatch doesn't satisfy the regex, return false
	if filter.RepositoryMatch != nil && !filter.RepositoryMatch.MatchString(repo.Repository) {
		return false
	}
	// If repository doesn't have the label set on labelMatch, return false
	if filter.LabelMatch != nil && !slices.ContainsFunc(repo.Labels, filter.LabelMatch.MatchString) {
		return false
	}

	return true
}

// matchBranchFilter reports whether repo/branch satisfies the branch-level
// conditions (BranchMatch, PathsExist, PathsDoNotExist) of a single filter.
// These conditions need branch data and may hit the SCM API, so they are only
// evaluated after the cheaper repo-level pre-filter in ListRepos. A filter with
// no branch-level conditions matches any branch.
func matchBranchFilter(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) (bool, error) {
	if filter.BranchMatch != nil && !filter.BranchMatch.MatchString(repo.Branch) {
		return false, nil
	}

	// Range over the path that the user has set in pathsExist and check if that path actually exists remotely
	// If the path doesn't exist, it means the repo doesn't satisfy branch conditions, hence return early
	for _, path := range filter.PathsExist {
		path = strings.TrimRight(path, "/")
		hasPath, err := provider.RepoHasPath(ctx, repo, path)
		if err != nil {
			return false, err
		}
		if !hasPath {
			return false, nil
		}
	}

	// We also need to make sure if a user has specified path that is not supposed to exist
	// Repo should satisfy that condition too
	for _, path := range filter.PathsDoNotExist {
		path = strings.TrimRight(path, "/")
		hasPath, err := provider.RepoHasPath(ctx, repo, path)
		if err != nil {
			return false, err
		}
		// The below line means if the repo has 'path that doesn't exist'
		// return false in that case
		if hasPath {
			return false, nil
		}
	}

	return true, nil
}

// ListRepos returns the repositories (expanded to one entry per branch) that
// match the given filters.
//
// Filter semantics (see docs/operator-manual/applicationset/Generators-SCM-Provider.md#filters):
//   - Conditions WITHIN a single filter are AND'd: all must pass.
//   - Multiple filters are OR'd: a repo/branch is kept if it fully satisfies
//     ANY one filter.
//   - No filters at all: every repo/branch is kept.
//
// A single filter may mix repo-level conditions (repositoryMatch, labelMatch)
// with branch-level conditions (branchMatch, pathsExist, pathsDoNotExist). Both
// halves of a filter must be checked against the SAME filter; that is the key
// to correct OR behavior. Evaluating all repo-level conditions and all
// branch-level conditions as two independent groups (the previous approach)
// accidentally AND'd unrelated filters together.
//
// Example — two filters:
//
//	filters:
//	- repositoryMatch: ^my-   # F0: repo-level only
//	- pathsExist: [helm]      # F1: branch-level only
//
// Repo "my-api" (no ./helm)  is kept because it fully satisfies F0.
// Repo "web"    (has ./helm) is kept because it fully satisfies F1.
// Neither repo needs to satisfy both filters, that is what OR means here.
//
// Evaluation runs in two passes purely for efficiency:
//  1. Repo-level pre-filter: drop repos that cannot satisfy ANY filter's
//     repo-level conditions, so we skip the expensive branch fetch for them.
//     A filter with no repo-level conditions matches every repo, so those
//     repos are always kept as candidates and decided in pass 2.
//  2. After fetching branches, keep a repo/branch if it fully satisfies at
//     least one filter. This pass re-checks the repo-level conditions, so
//     correctness never depends on the pre-filter, the pre-filter is only an
//     optimization.
func ListRepos(ctx context.Context, provider SCMProviderService, filters []argoprojiov1alpha1.SCMProviderGeneratorFilter, cloneProtocol string) ([]*Repository, error) {
	compiledFilters, err := compileFilters(filters)
	if err != nil {
		return nil, err
	}
	repos, err := provider.ListRepos(ctx, cloneProtocol)
	if err != nil {
		return nil, err
	}
	// OPTIMIZATION: pre-filter repos by their repo-level conditions so
	// we don't fetch branches for repos that cannot satisfy any filter. A repo is
	// a candidate if it matches the repo-level conditions of at least one filter
	// (a filter with no repo-level conditions matches every repo). With no
	// filters, all repos qualify.
	//
	// e.g. filters [repositoryMatch: ^my-] and [pathsExist: helm] make every repo
	// a candidate (the second filter has no repo-level condition), whereas
	// filters that are all repo-level let us discard non-matching repos up front.
	candidateRepos := repos
	if len(compiledFilters) > 0 {
		candidateRepos = make([]*Repository, 0, len(repos))
		for _, repo := range repos {
			for _, filter := range compiledFilters {
				if matchRepoFilter(repo, filter) {
					candidateRepos = append(candidateRepos, repo)
					break // candidate for at least one filter; stop checking the rest
				}
			}
		}
	}
	// Expand candidate repos into their branches
	reposWithBranches := make([]*Repository, 0, len(candidateRepos))
	for _, repo := range candidateRepos {
		filled, err := provider.GetBranches(ctx, repo)
		if err != nil {
			return nil, err
		}
		reposWithBranches = append(reposWithBranches, filled...)
	}
	if len(compiledFilters) == 0 {
		return reposWithBranches, nil
	}

	// Pass 2 (the actual filtering): include a repo/branch if it fully satisfies
	// at least one filter, i.e. all of that filter's repo-level AND branch-level
	// conditions. Checking both halves against the SAME filter, then OR'ing
	// across filters, is what gives the documented behavior.
	//
	// e.g. for filters [repositoryMatch: ^my-] and [pathsExist: helm]:
	//   - "my-api" (no ./helm) matches the first filter  -> kept
	//   - "web"    (has ./helm) matches the second filter -> kept
	filtered := make([]*Repository, 0, len(reposWithBranches))
	for _, repo := range reposWithBranches {
		for _, filter := range compiledFilters {
			// This filter's repo-level conditions fail; it cannot be the filter
			// that includes this repo, so move on to the next filter.
			if !matchRepoFilter(repo, filter) {
				continue
			}
			matches, err := matchBranchFilter(ctx, provider, repo, filter)
			if err != nil {
				return nil, err
			}
			if matches {
				// Fully satisfied this filter. Include the repo once and stop —
				// matching further filters wouldn't change the result (OR).
				filtered = append(filtered, repo)
				break
			}
		}
	}
	return filtered, nil
}
