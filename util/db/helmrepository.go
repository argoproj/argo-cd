package db

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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

func (db *db) getHelmRepo(repoURL string, helmRepositories []settings.HelmRepoCredentials) (*v1alpha1.Repository, error) {
	index := getHelmRepoCredIndex(helmRepositories, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	repoInfo := helmRepositories[index]

	repo := &v1alpha1.Repository{
		Repo: repoInfo.URL,
		Type: "helm",
		Name: repoInfo.Name,
	}
	err := db.unmarshalFromSecretsStr(map[*SecretMaperValidation]*v1.SecretKeySelector{
		&SecretMaperValidation{Dest: &repo.Username, Transform: StripCRLFCharacter}:          repoInfo.UsernameSecret,
		&SecretMaperValidation{Dest: &repo.Password, Transform: StripCRLFCharacter}:          repoInfo.PasswordSecret,
		&SecretMaperValidation{Dest: &repo.TLSClientCertData, Transform: StripCRLFCharacter}: repoInfo.CertSecret,
		&SecretMaperValidation{Dest: &repo.TLSClientCertKey, Transform: StripCRLFCharacter}:  repoInfo.KeySecret,
	}, make(map[string]*v1.Secret))
	return repo, err
}

// ListHelmRepoURLs lists configured helm repositories
func (db *db) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	helmRepositories, err := db.settingsMgr.GetHelmRepositories()
	if err != nil {
		return nil, err
	}

	result := make([]*v1alpha1.Repository, len(helmRepositories))
	for i, helmRepoInfo := range helmRepositories {
		repo, err := db.getHelmRepo(helmRepoInfo.URL, helmRepositories)
		if err != nil {
			return nil, err
		}
		result[i] = repo
	}
	repos, err := db.listRepositories(ctx, pointer.StringPtr("helm"))
	if err != nil {
		return nil, err
	}
	result = append(result, v1alpha1.Repositories(repos).Filter(func(r *v1alpha1.Repository) bool {
		return r.Type == "helm" && r.Name != ""
	})...)
	return result, nil
}
