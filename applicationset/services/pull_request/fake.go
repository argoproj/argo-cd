package pull_request

import (
	"context"
)

type FakeService struct {
	listPullReuests []*PullRequest
	listError       error
}

var _ PullRequestService = (*FakeService)(nil)

func NewFakeService(_ context.Context, listPullReuests []*PullRequest, listError error) (PullRequestService, error) {
	return &FakeService{
		listPullReuests: listPullReuests,
		listError:       listError,
	}, nil
}

func (g *FakeService) List(ctx context.Context) ([]*PullRequest, error) {
	return g.listPullReuests, g.listError
}
