package scm_provider

import (
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
)

func getOptionalHttpClient(optionalHttpClient ...*http.Client) *http.Client {
	if len(optionalHttpClient) > 0 && optionalHttpClient[0] != nil {
		return optionalHttpClient[0]
	}
	return &http.Client{}
}

func NewGithubAppProviderFor(g github_app_auth.Authentication, organization string, url string, allBranches bool, optionalHttpClient ...*http.Client) (*GithubProvider, error) {
	httpClient := getOptionalHttpClient(optionalHttpClient...)
	client, err := github_app.Client(g, url, httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}
