package scm_provider

import (
	"context"
	"net/http"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/github_app"
	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
)

func NewGithubAppProviderFor(ctx context.Context, g github_app_auth.Authentication, organization string, url string, allBranches bool, optionalHTTPClient ...*http.Client) (*GithubProvider, error) {
	httpClient := appsetutils.GetOptionalHTTPClient(optionalHTTPClient...)
	client, err := github_app.Client(ctx, g, url, organization, httpClient)
	if err != nil {
		return nil, err
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}
