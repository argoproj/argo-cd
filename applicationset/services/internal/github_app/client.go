package github_app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/util/git"
)

func getOptionalHTTPClientAndTransport(optionalHTTPClient ...*http.Client) (*http.Client, http.RoundTripper) {
	httpClient := appsetutils.GetOptionalHTTPClient(optionalHTTPClient...)
	if len(optionalHTTPClient) > 0 && optionalHTTPClient[0] != nil && optionalHTTPClient[0].Transport != nil {
		// will either use the provided custom httpClient and it's transport
		return httpClient, optionalHTTPClient[0].Transport
	}
	// or the default httpClient and transport
	return httpClient, http.DefaultTransport
}

// getInstallationClient creates a new GitHub client with the specified installation ID.
// It also returns a ghinstallation.Transport, which can be used for git requests.
func getInstallationClient(g github_app_auth.Authentication, url string) (*github.Client, error) {
	if g.InstallationId <= 0 {
		return nil, fmt.Errorf("installation ID is required for github")
	}

	itr, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
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
func Client(ctx context.Context, g github_app_auth.Authentication, url, org string, optionalHTTPClient ...*http.Client) (*github.Client, error) {
	if url == "" {
		url = g.EnterpriseBaseURL
	}

	// If an installation ID is already provided, use it directly.
	if g.InstallationId != 0 {
		return getInstallationClient(g, url)
	}

	// Auto-discover installation ID using shared utility
	// Pass optional HTTP client for metrics tracking
	installationId, err := git.DiscoverGitHubAppInstallationId(ctx, g.Id, g.PrivateKey, url, org, optionalHTTPClient...)
	if err != nil {
		return nil, err
	}

	g.InstallationId = installationId
	return getInstallationClient(g, url)

}
