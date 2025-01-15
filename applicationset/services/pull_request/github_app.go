package pull_request

import (
	"github.com/aburan28/httpcache"
	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
)

func NewGithubAppService(g github_app_auth.Authentication, url, owner, repo string, labels []string, cache httpcache.Cache) (PullRequestService, error) {
	client, err := github_app.Client(g, url, cache)
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
