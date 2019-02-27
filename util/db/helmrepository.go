package db

import (
	"context"
	"strings"

	apiv1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/util/settings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getHelmRepoCredIndex(s *settings.ArgoCDSettings, repoURL string) int {
	for i, cred := range s.HelmRepositories {
		if strings.ToLower(cred.URL) == strings.ToLower(repoURL) {
			return i
		}
	}
	return -1
}

func (db *db) getHelmRepo(ctx context.Context, repoURL string, s *settings.ArgoCDSettings) (*appv1.HelmRepository, error) {
	index := getHelmRepoCredIndex(s, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}

	helmRepoInfo := s.HelmRepositories[index]
	helmRepo := &appv1.HelmRepository{URL: repoURL, Name: helmRepoInfo.Name}
	cache := make(map[string]*apiv1.Secret)
	err := db.unmarshalFromSecretsBytes(map[*[]byte]*apiv1.SecretKeySelector{
		&helmRepo.CAData:   helmRepoInfo.CASecret,
		&helmRepo.CertData: helmRepoInfo.CertSecret,
		&helmRepo.KeyData:  helmRepoInfo.KeySecret,
	}, cache)
	if err != nil {
		return nil, err
	}
	err = db.unmarshalFromSecretsStr(map[*string]*apiv1.SecretKeySelector{
		&helmRepo.Username: helmRepoInfo.UsernameSecret,
		&helmRepo.Password: helmRepoInfo.PasswordSecret,
	}, cache)
	if err != nil {
		return nil, err
	}
	return helmRepo, nil
}

// ListHelmRepoURLs lists configured helm repositories
func (db *db) ListHelmRepos(ctx context.Context) ([]*appv1.HelmRepository, error) {
	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	repos := make([]*appv1.HelmRepository, len(s.HelmRepositories))
	for i, helmRepoInfo := range s.HelmRepositories {
		repo, err := db.getHelmRepo(ctx, helmRepoInfo.URL, s)
		if err != nil {
			return nil, err
		}
		repos[i] = repo
	}
	return repos, nil
}
