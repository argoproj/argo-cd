package scm_provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.SCMProviderGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.RepositoryMatch != nil {
			outFilter.RepositoryMatch, err = regexp.Compile(*filter.RepositoryMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling RepositoryMatch regexp %q: %v", *filter.RepositoryMatch, err)
			}
			outFilter.FilterTypeRepo = true
		}
		if filter.LabelMatch != nil {
			outFilter.LabelMatch, err = regexp.Compile(*filter.LabelMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling LabelMatch regexp %q: %v", *filter.LabelMatch, err)
			}
			outFilter.FilterTypeRepo = true
		}
		if filter.PathsExist != nil {
			outFilter.PathsExist = filter.PathsExist
			outFilter.FilterTypeBranch = true
		}
		if filter.PathsDoNotExist != nil {
			outFilter.PathsDoNotExist = filter.PathsDoNotExist
			outFilter.FilterTypeBranch = true
		}
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp.Compile(*filter.BranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %v", *filter.BranchMatch, err)
			}
			outFilter.FilterTypeBranch = true
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

func matchRepoFilter(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) bool {
	if filter.RepositoryMatch != nil && !filter.RepositoryMatch.MatchString(repo.Repository) {
		return false
	}

	if filter.LabelMatch != nil {
		found := false
		for _, label := range repo.Labels {
			if filter.LabelMatch.MatchString(label) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func matchBranchFilter(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) (bool, error) {
	if filter.BranchMatch != nil && !filter.BranchMatch.MatchString(repo.Branch) {
		return false, nil
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

	filledRepos := make([]*Repository, 0, len(repos))
	for _, repo := range repos {

		if len(compiledFilters) > 0 {
			for _, filter := range compiledFilters {
				if filter.FilterTypeRepo {
					if matches := matchRepoFilter(ctx, provider, repo, filter); !matches {
						continue
					}
				}

				filteredRepoWithBranches, err := getBranches(ctx, provider, repo, filter)
				if err != nil {
					return nil, err
				}

				filledRepos = append(filledRepos, filteredRepoWithBranches...)
			}
		} else {
			repoWithBranches, err := getBranches(ctx, provider, repo, nil)
			if err != nil {
				return nil, err
			}

			filledRepos = append(filledRepos, repoWithBranches...)
		}
	}

	return filledRepos, nil
}

func getBranches(ctx context.Context, provider SCMProviderService, repo *Repository, filter *Filter) ([]*Repository, error) {
	repoWithBranches := []*Repository{}

	repoFilled, err := provider.GetBranches(ctx, repo)
	if err != nil {
		return nil, err
	}
	repoWithBranches = append(repoWithBranches, repoFilled...)

	filteredRepoWithBranches := make([]*Repository, 0, len(repoWithBranches))
	if filter != nil && filter.FilterTypeBranch {
		for _, repoBranch := range repoWithBranches {
			matches, err := matchBranchFilter(ctx, provider, repoBranch, filter)
			if err != nil {
				return nil, err
			}
			if matches {
				filteredRepoWithBranches = append(filteredRepoWithBranches, repoBranch)
			}
		}
		return filteredRepoWithBranches, nil
	}

	return repoWithBranches, nil
}
