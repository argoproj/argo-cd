package pull_request

import (
	"context"
	"fmt"
	"regexp"
	"time"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.PullRequestGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp.Compile(*filter.BranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %w", *filter.BranchMatch, err)
			}
		}
		if filter.TargetBranchMatch != nil {
			outFilter.TargetBranchMatch, err = regexp.Compile(*filter.TargetBranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling TargetBranchMatch regexp %q: %w", *filter.TargetBranchMatch, err)
			}
		}
		if filter.TitleMatch != nil {
			outFilter.TitleMatch, err = regexp.Compile(*filter.TitleMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling TitleMatch regexp %q: %w", *filter.TitleMatch, err)
			}
		}
		if filter.CreatedWithin != nil {
			outFilter.CreatedWithin, err = time.ParseDuration(*filter.CreatedWithin)
			if err != nil {
				return nil, fmt.Errorf("error parsing CreatedWithin duration %s: %w", *filter.CreatedWithin, err)
			}
		}
		if filter.UpdatedWithin != nil {
			outFilter.UpdatedWithin, err = time.ParseDuration(*filter.UpdatedWithin)
			if err != nil {
				return nil, fmt.Errorf("error parsing UpdatedWithin duration %s: %w", *filter.UpdatedWithin, err)
			}
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

func matchFilter(pullRequest *PullRequest, filter *Filter) bool {
	if filter.BranchMatch != nil && !filter.BranchMatch.MatchString(pullRequest.Branch) {
		return false
	}
	if filter.TargetBranchMatch != nil && !filter.TargetBranchMatch.MatchString(pullRequest.TargetBranch) {
		return false
	}
	if filter.TitleMatch != nil && !filter.TitleMatch.MatchString(pullRequest.Title) {
		return false
	}
	if filter.CreatedWithin != 0 && pullRequest.CreatedAt.Before(time.Now().Add(-filter.CreatedWithin)) {
		return false
	}
	if filter.UpdatedWithin != 0 && pullRequest.UpdatedAt.Before(time.Now().Add(-filter.UpdatedWithin)) {
		return false
	}

	return true
}

func ListPullRequests(ctx context.Context, provider PullRequestService, filters []argoprojiov1alpha1.PullRequestGeneratorFilter) ([]*PullRequest, error) {
	compiledFilters, err := compileFilters(filters)
	if err != nil {
		return nil, err
	}

	pullRequests, err := provider.List(ctx)
	if err != nil {
		return nil, err
	}

	if len(compiledFilters) == 0 {
		return pullRequests, nil
	}

	filteredPullRequests := make([]*PullRequest, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		for _, filter := range compiledFilters {
			matches := matchFilter(pullRequest, filter)
			if matches {
				filteredPullRequests = append(filteredPullRequests, pullRequest)
				break
			}
		}
	}

	return filteredPullRequests, nil
}
