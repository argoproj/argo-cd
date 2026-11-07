package db

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ListHelmRepositories lists configured helm repositories
func (db *db) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	var result []*v1alpha1.Repository
	repos, err := db.listRepositories(ctx, new("helm"), false)
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm repositories: %w", err)
	}
	result = append(result, v1alpha1.Repositories(repos).Filter(func(r *v1alpha1.Repository) bool {
		// A classic Helm repository needs a name for `helm repo add`. An OCI registry is logged in to
		// by hostname, so it is listed without one; otherwise its credentials are never used for chart
		// dependencies.
		return r.Type == "helm" && (r.Name != "" || r.EnableOCI)
	})...)
	return result, nil
}
