package test

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type ArgoCDServiceMock struct {
	Mock *mock.Mock
}

func (a ArgoCDServiceMock) GetApps(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.Mock.Called(ctx, repoURL, revision)

	return args.Get(0).([]string), args.Error(1)
}

func (a ArgoCDServiceMock) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	args := a.Mock.Called(ctx, repoURL, revision, pattern)

	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (a ArgoCDServiceMock) GetFileContent(ctx context.Context, repoURL string, revision string, path string) ([]byte, error) {
	args := a.Mock.Called(ctx, repoURL, revision, path)

	return args.Get(0).([]byte), args.Error(1)
}

func (a ArgoCDServiceMock) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {
	args := a.Mock.Called(ctx, repoURL, revision)
	return args.Get(0).([]string), args.Error(1)
}
