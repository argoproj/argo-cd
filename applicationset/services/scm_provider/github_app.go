package scm_provider

import (
	"github.com/aburan28/httpcache"
	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/applicationset/services/internal/github_app"
)

func NewGithubAppProviderFor(g github_app_auth.Authentication, organization string, url string, allBranches bool, cache httpcache.Cache) (*GithubProvider, error) {
	client, err := github_app.Client(g, url, cache)
	if err != nil {
		return nil, err
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}
