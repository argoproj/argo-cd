package scm_provider

import (
	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/applicationset/services/internal/github_app"
)

func NewGithubAppProviderFor(g github_app_auth.Authentication, url string, options ...GithubOption) (*GithubProvider, error) {
	githubProvider := &GithubProvider{}
	client, err := github_app.Client(g, url)
	if err != nil {
		return nil, err
	}
	githubProvider.client = client

	for _, opt := range options {
		opt(githubProvider)
	}

	return githubProvider, nil
}
