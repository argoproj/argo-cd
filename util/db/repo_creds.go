package db

import (
	"context"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type RepoCredsDB interface {
	GetRepoCredsBySecretName(_ context.Context, secretName string) (*appsv1.RepoCreds, error)
}

func (db *db) GetRepoCredsBySecretName(ctx context.Context, secretName string) (*appsv1.RepoCreds, error) {
	return (&secretsRepositoryBackend{db: db}).GetRepoCredsBySecretName(ctx, secretName)
}
