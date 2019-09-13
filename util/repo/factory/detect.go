package factory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/helm/repo"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

func DetectType(r *v1alpha1.Repository, reporter metrics.Reporter) error {
	if r.Type != "" {
		return nil
	}
	_, err := repo.Index(r.Repo, r.Username, r.Password)
	if err == nil {
		r.Type = "helm"
		return nil
	}
	gitRepo, err := gitRepo(r, reporter)
	if err == nil {
		err = gitRepo.Init()
		if err == nil {
			r.Type = "git"
			return nil
		}
	}
	return err
}
