package factory

import (
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	helmrepo "github.com/argoproj/argo-cd/util/helm/repo"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

func DetectType(r *v1alpha1.Repository, reporter metrics.Reporter) error {
	log.WithField("repo", r).Info("DetectType")
	if r.Type != "" {
		return nil
	}
	_, err := helmrepo.Index(r.Repo, r.Username, r.Password)
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
