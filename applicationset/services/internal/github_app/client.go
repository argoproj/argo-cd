package github_app

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
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

// installationIds caches installation IDs for organizations to avoid redundant API calls.
var installationIds = make(map[orgAppId]int64)

// orgAppId is a composite key of organization and app ID for caching installation IDs.
type orgAppId struct {
	org string
	id  int64
}

// appInstallationIdCacheMutex protects access to the installationIds map.
var appInstallationIdCacheMutex sync.RWMutex

// Client builds a github client for the given app authentication.
func Client(ctx context.Context, g github_app_auth.Authentication, url, org string, optionalHTTPClient ...*http.Client) (*github.Client, error) {
	httpClient, transport := getOptionalHTTPClientAndTransport(optionalHTTPClient...)

	rt, err := ghinstallation.NewAppsTransport(transport, g.Id, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub installation transport: %w", err)
	}
	if url == "" {
		url = g.EnterpriseBaseURL
	}
	var client *github.Client
	httpClient.Transport = rt
	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		rt.BaseURL = url
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create github enterprise client: %w", err)
		}
	}

	// If an installation ID is already provided, use it directly.
	if g.InstallationId != 0 {
		return getInstallationClient(g, url)
	}

	appInstallationIdCacheMutex.RLock()
	if id, found := installationIds[orgAppId{org: org, id: g.Id}]; found {
		appInstallationIdCacheMutex.RUnlock()
		g.InstallationId = id
		return getInstallationClient(g, url)
	}
	appInstallationIdCacheMutex.RUnlock()

	var allInstallations []*github.Installation
	opts := &github.ListOptions{PerPage: 100}

	// Cache the installation IDs, we take out a lock for the entire loop to avoid locking/unlocking repeatedly. We also include the single
	// read within the write lock.
	// This lock should also help with the fact that on restart we won't slam the GitHub API with multiple requests to list installations.
	appInstallationIdCacheMutex.Lock()
	for {
		installations, resp, err := client.Apps.ListInstallations(ctx, opts)
		if err != nil {
			appInstallationIdCacheMutex.Unlock()
			return nil, fmt.Errorf("failed to list installations: %w", err)
		}

		allInstallations = append(allInstallations, installations...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	for _, installation := range allInstallations {
		if installation.Account != nil && installation.Account.Login != nil && installation.ID != nil {
			installationIds[orgAppId{org: *installation.Account.Login, id: g.Id}] = *installation.ID
		}
	}

	if id, found := installationIds[orgAppId{org: org, id: g.Id}]; found {
		appInstallationIdCacheMutex.Unlock()
		g.InstallationId = id
		return getInstallationClient(g, url)
	}
	appInstallationIdCacheMutex.Unlock()
	return nil, fmt.Errorf("installation not found for org: %s", org)

}
