package github_app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/util/git"
)

// getInstallationClient creates a new GitHub client with the specified installation ID.
// It also returns a ghinstallation.Transport, which can be used for git requests.
// When enableTokenCache is true, token acquisition requests are intercepted so that
// a valid installation access token is reused for its lifetime instead of being
// refetched on every call.
func getInstallationClient(g github_app_auth.Authentication, url string, enableTokenCache bool, httpClient ...*http.Client) (*github.Client, error) {
	if g.InstallationId <= 0 {
		return nil, errors.New("installation ID is required for github")
	}

	// Use provided HTTP client's transport or default
	var transport http.RoundTripper
	if len(httpClient) > 0 && httpClient[0] != nil && httpClient[0].Transport != nil {
		transport = httpClient[0].Transport
	} else {
		transport = http.DefaultTransport
	}

	// Optionally add caching layer to avoid refetching the installation access token
	// on every call. Gated by the same flag as the HTTP response cache so operators
	// can opt in or out of both caching layers together.
	if enableTokenCache {
		transport = NewGitHubAppCacheTokenTransport(transport, g.Id, g.InstallationId)
	}

	itr, err := ghinstallation.New(transport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub installation transport: %w", err)
	}

	if url == "" {
		url = g.EnterpriseBaseURL
	}

	var client *github.Client
	if url == "" {
		client = github.NewClient(&http.Client{Transport: itr})
		return client, nil
	}

	itr.BaseURL = url
	client, err = github.NewClient(&http.Client{Transport: itr}).WithEnterpriseURLs(url, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub enterprise client: %w", err)
	}
	return client, nil
}

// Client builds a github client for the given app authentication.
// enableTokenCache controls whether the installation access token is cached for
// its lifetime (up to ~1 h) to avoid a token-acquisition round-trip on every call.
// Mandatory if --enable-github-cache is set on the ApplicationSet controller.
func Client(ctx context.Context, g github_app_auth.Authentication, url, org string, enableTokenCache bool, optionalHTTPClient ...*http.Client) (*github.Client, error) {
	if url == "" {
		url = g.EnterpriseBaseURL
	}

	// If an installation ID is already provided, use it directly.
	if g.InstallationId != 0 {
		return getInstallationClient(g, url, enableTokenCache, optionalHTTPClient...)
	}

	// Auto-discover installation ID using shared utility
	// Pass optional HTTP client for metrics tracking
	installationId, err := git.DiscoverGitHubAppInstallationID(ctx, g.Id, g.PrivateKey, url, org, optionalHTTPClient...)
	if err != nil {
		return nil, err
	}

	g.InstallationId = installationId
	return getInstallationClient(g, url, enableTokenCache, optionalHTTPClient...)
}
