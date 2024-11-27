package pull_request

import (
	"context"
	"regexp"
)

type PullRequest struct {
	// Number is a number that will be the ID of the pull request.
	Number int
	// Title of the pull request.
	Title string
	// Branch is the name of the branch from which the pull request originated.
	Branch string
	// TargetBranch is the name of the target branch of the pull request.
	TargetBranch string
	// HeadSHA is the SHA of the HEAD from which the pull request originated.
	HeadSHA string
	// Labels of the pull request.
	Labels []string
	// Author is the author of the pull request.
	Author string
}

type PullRequestService interface {
	// List gets a list of pull requests.
	List(ctx context.Context) ([]*PullRequest, error)
}

type Filter struct {
	BranchMatch       *regexp.Regexp
	TargetBranchMatch *regexp.Regexp
}
