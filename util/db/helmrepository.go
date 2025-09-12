package db

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ListHelmRepositories lists configured helm repositories
func (db *db) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	var result []*v1alpha1.Repository
	repos, err := db.listRepositories(ctx, ptr.To("helm"), false)
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm repositories: %w", err)
	}
	result = append(result, v1alpha1.Repositories(repos).Filter(func(r *v1alpha1.Repository) bool {
		return r.Type == "helm" && r.Name != ""
	})...)
	return result, nil
}
