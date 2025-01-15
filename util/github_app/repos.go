package github_app

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/util/db"
)

// NewAuthCredentials returns a GtiHub App credentials lookup by repo-creds url.
func NewAuthCredentials(creds db.RepoCredsDB) github_app_auth.Credentials {
	return &repoAsCredentials{RepoCredsDB: creds}
}

type repoAsCredentials struct {
	db.RepoCredsDB
}

func (r *repoAsCredentials) GetAuthSecret(ctx context.Context, secretName string) (*github_app_auth.Authentication, error) {
	repo, err := r.GetRepoCredsBySecretName(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("error getting creds for %s: %w", secretName, err)
	}
	if repo == nil || repo.GithubAppPrivateKey == "" {
		return nil, fmt.Errorf("no github app found for %s", secretName)
	}
	return &github_app_auth.Authentication{
		Id:                repo.GithubAppId,
		InstallationId:    repo.GithubAppInstallationId,
		EnterpriseBaseURL: repo.GitHubAppEnterpriseBaseURL,
		PrivateKey:        repo.GithubAppPrivateKey,
	}, nil
}

var _ github_app_auth.Credentials = (*repoAsCredentials)(nil)
