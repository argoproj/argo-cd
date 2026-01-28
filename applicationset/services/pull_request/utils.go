package pull_request

import (
	"context"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func compileTimeDurationFilter(filterValue *string, output **time.Duration, filterName string) error {
	if filterValue == nil {
		return nil
	}

	d, err := time.ParseDuration(*filterValue)
	if err != nil {
		return fmt.Errorf("error parsing %s duration %s: %w", filterName, *filterValue, err)
	}

	*output = &d

	return nil
}

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
		if err := compileTimeDurationFilter(filter.CreatedWithin, &outFilter.CreatedWithin, "CreatedWithin"); err != nil {
			return nil, err
		}
		if err := compileTimeDurationFilter(filter.UpdatedWithin, &outFilter.UpdatedWithin, "UpdatedWithin"); err != nil {
			return nil, err
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
	if pullRequest.IsCreatedWithin(filter.CreatedWithin) {
		return false
	}
	if pullRequest.IsUpdatedWithin(filter.UpdatedWithin) {
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
		matches := false
		for _, filter := range compiledFilters {
			matches = matchFilter(pullRequest, filter)
			if matches {
				filteredPullRequests = append(filteredPullRequests, pullRequest)
				break
			}
		}

		if !matches {
			log.WithFields(log.Fields{
				"pr":              pullRequest,
				"applied_filters": compiledFilters,
			}).Debug("pull request filtered out")
		}
	}

	return filteredPullRequests, nil
}
