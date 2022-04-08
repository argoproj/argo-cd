package pull_request

import "context"

type PullRequest struct {
	// Number is a number that will be the ID of the pull request.
	Number int
	// Branch is the name of the branch from which the pull request originated.
	Branch string
	// HeadSHA is the SHA of the HEAD from which the pull request originated.
	HeadSHA string
}

type PullRequestService interface {
	// List gets a list of pull requests.
	List(ctx context.Context) ([]*PullRequest, error)
}
