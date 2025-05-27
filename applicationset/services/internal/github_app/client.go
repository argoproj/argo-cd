package github_app

import (
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
)

func getOptionalHttpClientAndTransport(optionalHttpClient ...*http.Client) (*http.Client, http.RoundTripper) {
	if len(optionalHttpClient) > 0 && optionalHttpClient[0] != nil {
		// will either use the provided custom httpClient and it's transport
		return optionalHttpClient[0], optionalHttpClient[0].Transport
	}
	// or the default httpClient and transport
	return &http.Client{}, http.DefaultTransport
}

// Client builds a github client for the given app authentication.
func Client(g github_app_auth.Authentication, url string, optionalHttpClient ...*http.Client) (*github.Client, error) {
	httpClient, transport := getOptionalHttpClientAndTransport(optionalHttpClient...)

	rt, err := ghinstallation.New(transport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app install: %w", err)
	}
	if url == "" {
		url = g.EnterpriseBaseURL
	}
	var client *github.Client
	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		rt.BaseURL = url
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create github enterprise client: %w", err)
		}
	}
	return client, nil
}
