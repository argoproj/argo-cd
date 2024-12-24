package github_app

import (
	"fmt"
	"net/http"

	"github.com/aburan28/httpcache"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v66/github"

	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
)

// Client builds a github client for the given app authentication.
func Client(g github_app_auth.Authentication, url string, cache httpcache.Cache) (*github.Client, error) {
	var httpClient *http.Client
	var err error
	var client *github.Client

	if cache != nil {
		tr := httpcache.NewTransport(cache)
		at, err := ghinstallation.NewAppsTransport(tr, g.Id, []byte(g.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app transport: %w", err)
		}

		httpClient = &http.Client{
			Transport: at,
		}
	} else {
		rt, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app install: %w", err)
		}
		httpClient = &http.Client{
			Transport: rt,
		}

	}

	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}
	}
	return client, nil
}
