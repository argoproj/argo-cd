package pull_request

import (
	"context"
	"regexp"
	"time"
)

type PullRequest struct {
	// Number is a number that will be the ID of the pull request.
	// Gitlab uses int64 for the pull request number.
	Number int64
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
	// Time when pull request was created
	CreatedAt time.Time
	// Time when pull request was updated
	UpdatedAt time.Time
}

func (p PullRequest) IsCreatedWithin(t *time.Duration) bool {
	return t != nil && p.CreatedAt.Before(time.Now().UTC().Add(-*t))
}

func (p PullRequest) IsUpdatedWithin(t *time.Duration) bool {
	return t != nil && p.UpdatedAt.Before(time.Now().UTC().Add(-*t))
}

type PullRequestService interface {
	// List gets a list of pull requests.
	List(ctx context.Context) ([]*PullRequest, error)
}

type Filter struct {
	BranchMatch       *regexp.Regexp
	TargetBranchMatch *regexp.Regexp
	TitleMatch        *regexp.Regexp
	CreatedWithin     *time.Duration
	UpdatedWithin     *time.Duration
}
