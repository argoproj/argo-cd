package github_app

import (
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
)

func getOptionalHTTPClientAndTransport(optionalHTTPClient ...*http.Client) (*http.Client, http.RoundTripper) {
	if len(optionalHTTPClient) > 0 && optionalHTTPClient[0] != nil && optionalHTTPClient[0].Transport != nil {
		// will either use the provided custom httpClient and it's transport
		return optionalHTTPClient[0], optionalHTTPClient[0].Transport
	}
	// or the default httpClient and transport
	return &http.Client{}, http.DefaultTransport
}

// Client builds a github client for the given app authentication.
func Client(g github_app_auth.Authentication, url string, optionalHTTPClient ...*http.Client) (*github.Client, error) {
	_, transport := getOptionalHTTPClientAndTransport(optionalHTTPClient...)

	rt, err := ghinstallation.New(transport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app install: %w", err)
	}
	if url == "" {
		url = g.EnterpriseBaseURL
	}
	var client *github.Client
	if url == "" {
		client = github.NewClient(&http.Client{Transport: rt})
	} else {
		rt.BaseURL = url
		client, err = github.NewClient(&http.Client{Transport: rt}).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create github enterprise client: %w", err)
		}
	}
	return client, nil
}
