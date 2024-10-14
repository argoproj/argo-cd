package db

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func (db *db) GetWriteCredentials(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	secret, err := db.getRepoCredsSecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, err
	}

	return secretToRepository(secret)
}

func (db *db) getRepoCredsSecret(repoURL string) (*corev1.Secret, error) {
	// Should reuse stuff from repo secrets backend...
	secretBackend := &secretsRepositoryBackend{db: db}

	secrets, err := db.listSecretsByType(common.LabelValueSecretTypeRepositoryWrite)
	if err != nil {
		return nil, err
	}

	index := secretBackend.getRepositoryCredentialIndex(secrets, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repository credentials %q not found", repoURL)
	}

	return secrets[index], nil
}
