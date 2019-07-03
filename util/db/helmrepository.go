package db

import (
	"context"
	"strings"

	apiv1 "k8s.io/api/core/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

func getHelmRepoCredIndex(helmRepositories []settings.HelmRepoCredentials, repoURL string) int {
	for i, cred := range helmRepositories {
		if strings.EqualFold(cred.URL, repoURL) {
			return i
		}
	}
	return -1
}

func (db *db) getHelmRepo(repoURL string, helmRepositories []settings.HelmRepoCredentials) (*appv1.HelmRepository, error) {
	index := getHelmRepoCredIndex(helmRepositories, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}

	helmRepoInfo := helmRepositories[index]
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
	helmRepositories, err := db.settingsMgr.GetHelmRepositories()
	if err != nil {
		return nil, err
	}

	repos := make([]*appv1.HelmRepository, len(helmRepositories))
	for i, helmRepoInfo := range helmRepositories {
		repo, err := db.getHelmRepo(helmRepoInfo.URL, helmRepositories)
		if err != nil {
			return nil, err
		}
		repos[i] = repo
	}
	return repos, nil
}
