package scm_provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.SCMProviderGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.RepositoryMatch != nil {
			outFilter.RepositoryMatch, err = regexp2.Compile(*filter.RepositoryMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling RepositoryMatch regexp %q: %w", *filter.RepositoryMatch, err)
			}
			outFilter.FilterType = FilterTypeRepo
		}
		if filter.LabelMatch != nil {
			outFilter.LabelMatch, err = regexp2.Compile(*filter.LabelMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling LabelMatch regexp %q: %w", *filter.LabelMatch, err)
			}
			outFilter.FilterType = FilterTypeRepo
		}
		if filter.PathsExist != nil {
			outFilter.PathsExist = filter.PathsExist
			outFilter.FilterType = FilterTypeBranch
		}
		if filter.PathsDoNotExist != nil {
			outFilter.PathsDoNotExist = filter.PathsDoNotExist
			outFilter.FilterType = FilterTypeBranch
		}
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp2.Compile(*filter.BranchMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %w", *filter.BranchMatch, err)
			}
			outFilter.FilterType = FilterTypeBranch
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

// matchRegexp reports whether re matches text, treating a match-time error as a non-match.
func matchRegexp(re *regexp2.Regexp, text string) bool {
	matched, err := re.MatchString(text)
	if err != nil {
		log.Warnf("failed to match pattern %s due to error %v", re.String(), err)
		return false
	}
	return matched
}

func matchFilter(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) (bool, error) {
	if filter.RepositoryMatch != nil && !matchRegexp(filter.RepositoryMatch, repo.Repository) {
		return false, nil
	}

	if filter.BranchMatch != nil && !matchRegexp(filter.BranchMatch, repo.Branch) {
		return false, nil
	}

	if filter.LabelMatch != nil {
		found := slices.ContainsFunc(repo.Labels, func(label string) bool {
			return matchRegexp(filter.LabelMatch, label)
		})
		if !found {
			return false, nil
		}
	}

	if len(filter.PathsExist) != 0 {
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
	}
	if len(filter.PathsDoNotExist) != 0 {
		for _, path := range filter.PathsDoNotExist {
			path = strings.TrimRight(path, "/")
			hasPath, err := provider.RepoHasPath(ctx, repo, path)
			if err != nil {
				return false, err
			}
			if hasPath {
				return false, nil
			}
		}
	}

	return true, nil
}

func ListRepos(ctx context.Context, provider SCMProviderService, filters []argoprojiov1alpha1.SCMProviderGeneratorFilter, cloneProtocol string) ([]*Repository, error) {
	compiledFilters, err := compileFilters(filters)
	if err != nil {
		return nil, err
	}
	repos, err := provider.ListRepos(ctx, cloneProtocol)
	if err != nil {
		return nil, err
	}
	repoFilters := getApplicableFilters(compiledFilters)[FilterTypeRepo]
	if len(repoFilters) == 0 {
		repos, err := getBranches(ctx, provider, repos, compiledFilters)
		if err != nil {
			return nil, err
		}
		return repos, nil
	}
	filteredRepos := make([]*Repository, 0, len(repos))
	for _, repo := range repos {
		for _, filter := range repoFilters {
			matches, err := matchFilter(ctx, provider, repo, filter)
			if err != nil {
				return nil, err
			}
			if matches {
				filteredRepos = append(filteredRepos, repo)
				break
			}
		}
	}

	repos, err = getBranches(ctx, provider, filteredRepos, compiledFilters)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func getBranches(ctx context.Context, provider SCMProviderService, repos []*Repository, compiledFilters []*Filter) ([]*Repository, error) {
	reposWithBranches := []*Repository{}
	for _, repo := range repos {
		reposFilled, err := provider.GetBranches(ctx, repo)
		if err != nil {
			return nil, err
		}
		reposWithBranches = append(reposWithBranches, reposFilled...)
	}
	branchFilters := getApplicableFilters(compiledFilters)[FilterTypeBranch]
	if len(branchFilters) == 0 {
		return reposWithBranches, nil
	}
	filteredRepos := make([]*Repository, 0, len(reposWithBranches))
	for _, repo := range reposWithBranches {
		for _, filter := range branchFilters {
			matches, err := matchFilter(ctx, provider, repo, filter)
			if err != nil {
				return nil, err
			}
			if matches {
				filteredRepos = append(filteredRepos, repo)
				break
			}
		}
	}
	return filteredRepos, nil
}

// getApplicableFilters returns a map of filters separated by type.
func getApplicableFilters(filters []*Filter) map[FilterType][]*Filter {
	filterMap := map[FilterType][]*Filter{
		FilterTypeBranch: {},
		FilterTypeRepo:   {},
	}
	for _, filter := range filters {
		switch filter.FilterType {
		case FilterTypeBranch:
			filterMap[FilterTypeBranch] = append(filterMap[FilterTypeBranch], filter)
		case FilterTypeRepo:
			filterMap[FilterTypeRepo] = append(filterMap[FilterTypeRepo], filter)
		}
	}
	return filterMap
}
