package pull_request

import (
	"fmt"
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/shurcooL/githubv4"
)

func NewGithubAppService(g github_app_auth.Authentication, url, owner, repo string, labels []string) (PullRequestService, error) {
	rt, err := ghinstallation.New(http.DefaultTransport, g.Id, g.InstallationId, []byte(g.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create github app install: %w", err)
	}
	if url == "" {
		url = g.EnterpriseBaseURL
	}
	var client *githubv4.Client
	if url == "" {
		httpClient := http.Client{Transport: rt}
		client = githubv4.NewClient(&httpClient)
	} else {
		rt.BaseURL = url
		httpClient := http.Client{Transport: rt}
		client = githubv4.NewEnterpriseClient(url, &httpClient)
	}
	return &GithubService{
		client: client,
		owner:  owner,
		repo:   repo,
		labels: labels,
	}, nil
}
