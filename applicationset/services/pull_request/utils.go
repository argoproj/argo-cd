package pull_request

import (
	"context"
	"fmt"

	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func compileFilters(filters []argoprojiov1alpha1.PullRequestGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regexp2.Compile(*filter.BranchMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %w", *filter.BranchMatch, err)
			}
		}
		if filter.TargetBranchMatch != nil {
			outFilter.TargetBranchMatch, err = regexp2.Compile(*filter.TargetBranchMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling TargetBranchMatch regexp %q: %w", *filter.TargetBranchMatch, err)
			}
		}
		if filter.TitleMatch != nil {
			outFilter.TitleMatch, err = regexp2.Compile(*filter.TitleMatch, 0)
			if err != nil {
				return nil, fmt.Errorf("error compiling TitleMatch regexp %q: %w", *filter.TitleMatch, err)
			}
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

func matchFilter(pullRequest *PullRequest, filter *Filter) bool {
	if filter.BranchMatch != nil && !matchRegexp(filter.BranchMatch, pullRequest.Branch) {
		return false
	}
	if filter.TargetBranchMatch != nil && !matchRegexp(filter.TargetBranchMatch, pullRequest.TargetBranch) {
		return false
	}
	if filter.TitleMatch != nil && !matchRegexp(filter.TitleMatch, pullRequest.Title) {
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
