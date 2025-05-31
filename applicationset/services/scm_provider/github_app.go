package scm_provider

import (
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
)

func getOptionalHTTPClient(optionalHTTPClient ...*http.Client) *http.Client {
	if len(optionalHTTPClient) > 0 && optionalHTTPClient[0] != nil {
		return optionalHTTPClient[0]
	}
	return &http.Client{}
}

func NewGithubAppProviderFor(g github_app_auth.Authentication, organization string, url string, allBranches bool, optionalHTTPClient ...*http.Client) (*GithubProvider, error) {
	httpClient := getOptionalHTTPClient(optionalHTTPClient...)
	client, err := github_app.Client(g, url, httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}
