package factory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/app/discovery"
	"github.com/argoproj/argo-cd/util/creds"
	gitrepo "github.com/argoproj/argo-cd/util/git/repo"
	helmrepo "github.com/argoproj/argo-cd/util/helm/repo"
	"github.com/argoproj/argo-cd/util/repo"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

type Factory interface {
	NewRepo(r *v1alpha1.Repository, reporter metrics.Reporter) (repo.Repo, error)
}

func NewFactory() Factory {
	return &factory{}
}

type factory struct {
}

func (f *factory) NewRepo(r *v1alpha1.Repository, reporter metrics.Reporter) (repo.Repo, error) {
	switch r.Type {
	case "helm":
		return helmRepo(r)
	default:
		return gitRepo(r, reporter)
	}
}

func gitRepo(r *v1alpha1.Repository, reporter metrics.Reporter) (repo.Repo, error) {
	return gitrepo.NewRepo(r.Repo, creds.GetRepoCreds(r), r.IsInsecure(), r.EnableLFS, discovery.Discover, reporter)
}

func helmRepo(r *v1alpha1.Repository) (repo.Repo, error) {
	return helmrepo.NewRepo(r.Repo, r.Name, r.Username, r.Password, []byte(r.TLSClientCAData), []byte(r.TLSClientCertData), []byte(r.TLSClientCertKey))
}
