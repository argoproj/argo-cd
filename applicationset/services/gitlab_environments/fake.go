package gitlab_environments

import (
	"context"
)

type FakeService struct {
	listEnvironments []*Environment
	listError        error
}

var _ EnvironmentService = (*FakeService)(nil)

func NewFakeService(_ context.Context, listEnvironments []*Environment, listError error) (EnvironmentService, error) {
	return &FakeService{
		listEnvironments: listEnvironments,
		listError:        listError,
	}, nil
}

func (g *FakeService) List(ctx context.Context) ([]*Environment, error) {
	return g.listEnvironments, g.listError
}
