package pull_request

import (
	"context"
	"fmt"

	"github.com/dlclark/regexp2"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/regex"
)

func compileFilters(filters []argoprojiov1alpha1.PullRequestGeneratorFilter) ([]*Filter, error) {
	outFilters := make([]*Filter, 0, len(filters))
	for _, filter := range filters {
		outFilter := &Filter{}
		var err error
		if filter.BranchMatch != nil {
			outFilter.BranchMatch, err = regex.CompileFilter(*filter.BranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling BranchMatch regexp %q: %w", *filter.BranchMatch, err)
			}
		}
		if filter.TargetBranchMatch != nil {
			outFilter.TargetBranchMatch, err = regex.CompileFilter(*filter.TargetBranchMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling TargetBranchMatch regexp %q: %w", *filter.TargetBranchMatch, err)
			}
		}
		if filter.TitleMatch != nil {
			outFilter.TitleMatch, err = regex.CompileFilter(*filter.TitleMatch)
			if err != nil {
				return nil, fmt.Errorf("error compiling TitleMatch regexp %q: %w", *filter.TitleMatch, err)
			}
		}
		outFilters = append(outFilters, outFilter)
	}
	return outFilters, nil
}

func matchFilter(pullRequest *PullRequest, filter *Filter) (bool, error) {
	for _, m := range []struct {
		re    *regexp2.Regexp
		field string
		text  string
	}{
		{filter.BranchMatch, "BranchMatch", pullRequest.Branch},
		{filter.TargetBranchMatch, "TargetBranchMatch", pullRequest.TargetBranch},
		{filter.TitleMatch, "TitleMatch", pullRequest.Title},
	} {
		if m.re == nil {
			continue
		}
		matched, err := m.re.MatchString(m.text)
		if err != nil {
			return false, fmt.Errorf("error matching %s regexp %q: %w", m.field, m.re.String(), err)
		}
		if !matched {
			return false, nil
		}
	}

	return true, nil
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
			matches, err := matchFilter(pullRequest, filter)
			if err != nil {
				return nil, fmt.Errorf("error filtering pull request %d: %w", pullRequest.Number, err)
			}
			if matches {
				filteredPullRequests = append(filteredPullRequests, pullRequest)
				break
			}
		}
	}

	return filteredPullRequests, nil
}
