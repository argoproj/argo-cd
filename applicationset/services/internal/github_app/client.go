package github_app

import (
	"fmt"
	"net/http"

	"github.com/aburan28/httpcache"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v66/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
)

func CachedClient(cache httpcache.Cache) *http.Client {
	httpClient := http.Client{}
	httpClient = http.Client{Transport: &httpcache.Transport{Cache: cache}}
	return &httpClient
}

func SetupAppAuthTransport() {

}

// Client builds a github client for the given app authentication.
// this should return the following clients given inputs
// 1. url empty and cache nil
// 2. url empty and cache not nil
// 3. url not empty and cache nil
// 4. url not empty and cache not nil
func Client(g github_app_auth.Authentication, url string, cache httpcache.Cache) (*github.Client, error) {
	var (
		rt  http.RoundTripper
		err error
	)
	httpClient := &http.Client{}
	// if cache is not nil, create a new http client with cache transport
	if cache != nil {
		httpClient = CachedClient(cache)

		rt, err = ghinstallation.New(httpClient.Transport, g.Id, g.InstallationId, []byte(g.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app transport: %w", err)
		}
	} else {
		rt, err = ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app install: %w", err)
		}
	}

	// set httpClient to use Transport from above. If cache used it will use the cache transport else it will use default transport
	httpClient.Transport = rt
	// Create the GitHub client.
	if url == "" {
		return github.NewClient(httpClient), nil
	}

	client, err := github.NewClient(httpClient).WithEnterpriseURLs(url, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
	}
	return client, nil
}
