package github_app

import (
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v63/github"

	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
)

// Client builds a github client for the given app authentication.
func Client(g github_app_auth.Authentication, url string) (*github.Client, error) {
	rt, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app install: %w", err)
	}
	if url == "" {
		url = g.EnterpriseBaseURL
	}
	var client *github.Client
	if url == "" {
		httpClient := http.Client{Transport: rt}
		client = github.NewClient(&httpClient)
	} else {
		rt.BaseURL = url
		httpClient := http.Client{Transport: rt}
		client, err = github.NewClient(&httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create github enterprise client: %w", err)
		}
	}
	return client, nil
}
