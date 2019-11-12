package settings

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
)

type StaticCredsStore struct {
	Clusters map[string]v1alpha1.Cluster
	Repos    map[string]v1alpha1.Repository
}

func (s *StaticCredsStore) GetCluster(ctx context.Context, url string) (*v1alpha1.Cluster, error) {
	if cluster, ok := s.Clusters[url]; ok {
		return &cluster, nil
	}
	return nil, fmt.Errorf("cluster %s not found", url)
}

func (s *StaticCredsStore) GetRepository(ctx context.Context, url string) (*v1alpha1.Repository, error) {
	if repo, ok := s.Repos[url]; ok {
		return &repo, nil
	}
	return &v1alpha1.Repository{Repo: url}, nil
}

func (s *StaticCredsStore) WatchClusters(ctx context.Context, callback func(event *pkg.ClusterEvent)) error {
	return nil
}
func (s *StaticCredsStore) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	res := make([]*v1alpha1.Repository, 0)
	for url := range s.Repos {
		repo := s.Repos[url]
		if repo.Type == "helm" {
			res = append(res, &repo)
		}
	}
	return res, nil
}
