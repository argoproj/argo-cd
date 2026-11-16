package pull_request

import (
	"context"
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
)

func NewGithubAppService(ctx context.Context, g github_app_auth.Authentication, url, owner, repo string, labels []string, excludedLabels []string, optionalHTTPClient ...*http.Client) (PullRequestService, error) {
	httpClient := appsetutils.GetOptionalHTTPClient(optionalHTTPClient...)
	client, err := github_app.Client(ctx, g, url, owner, httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubService{
		client:         client,
		owner:          owner,
		repo:           repo,
		labels:         labels,
		excludedLabels: excludedLabels,
	}, nil
}
