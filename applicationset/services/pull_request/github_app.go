package pull_request

import (
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
)

func NewGithubAppService(g github_app_auth.Authentication, url, owner, repo string, labels []string, httpClient *http.Client) (PullRequestService, error) {
	client, err := github_app.Client(g, url, *httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubService{
		client: client,
		owner:  owner,
		repo:   repo,
		labels: labels,
	}, nil
}
