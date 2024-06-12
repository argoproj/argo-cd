package commit

import (
	"fmt"
	"net/http"

	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v62/github"
)

func isGitHubApp(cred *v1alpha1.Repository) bool {
	return cred.GithubAppPrivateKey != "" && cred.GithubAppId != 0 && cred.GithubAppInstallationId != 0
}

// Client builds a github client for the given app authentication.
func getAppInstallation(g github_app_auth.Authentication) (*ghinstallation.Transport, error) {
	rt, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app install: %w", err)
	}
	return rt, nil
}

func getGitHubInstallationClient(rt *ghinstallation.Transport) *github.Client {
	httpClient := http.Client{Transport: rt}
	client := github.NewClient(&httpClient)
	return client
}

func getGitHubAppClient(g github_app_auth.Authentication) (*github.Client, error) {
	var client *github.Client
	var err error

	// This creates the app authenticated with the bearer JWT, not the installation token.
	rt, err := ghinstallation.NewAppsTransport(http.DefaultTransport, g.Id, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app: %w", err)
	}

	httpClient := http.Client{Transport: rt}
	client = github.NewClient(&httpClient)
	return client, err
}
