package db

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
		{Dest: &repo.Username, Transform: StripCRLFCharacter}:          repoInfo.UsernameSecret,
		{Dest: &repo.Password, Transform: StripCRLFCharacter}:          repoInfo.PasswordSecret,
		{Dest: &repo.TLSClientCertData, Transform: StripCRLFCharacter}: repoInfo.CertSecret,
		{Dest: &repo.TLSClientCertKey, Transform: StripCRLFCharacter}:  repoInfo.KeySecret,
	}, make(map[string]*v1.Secret))
	return repo, err
}

// ListHelmRepositories lists configured helm repositories
func (db *db) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	helmRepositories, err := db.settingsMgr.GetHelmRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of Helm repositories from settings manager: %w", err)
	}

	result := make([]*v1alpha1.Repository, len(helmRepositories))
	for i, helmRepoInfo := range helmRepositories {
		repo, err := db.getHelmRepo(helmRepoInfo.URL, helmRepositories)
		if err != nil {
			return nil, fmt.Errorf("failed to get Helm repository %q: %w", helmRepoInfo.URL, err)
		}
		result[i] = repo
	}
	repos, err := db.listRepositories(ctx, ptr.To("helm"))
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm repositories: %w", err)
	}
	result = append(result, v1alpha1.Repositories(repos).Filter(func(r *v1alpha1.Repository) bool {
		return r.Type == "helm" && r.Name != ""
	})...)
	return result, nil
}
