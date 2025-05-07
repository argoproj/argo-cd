package scm_provider

import (
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
)

func NewGithubAppProviderFor(g github_app_auth.Authentication, organization string, url string, allBranches bool, httpClient *http.Client) (*GithubProvider, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	client, err := github_app.Client(g, url, *httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}
