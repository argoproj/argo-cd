package factory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/app/disco"
	"github.com/argoproj/argo-cd/util/creds"
	repo2 "github.com/argoproj/argo-cd/util/git/repo"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/repo"
)

type Factory interface {
	NewRepo(r *v1alpha1.Repository) (repo.Repo, error)
}

func NewFactory() Factory {
	return &factory{}
}

type factory struct {
}

func (f *factory) NewRepo(r *v1alpha1.Repository) (repo.Repo, error) {
	switch r.Type {
	case "helm":
		return helm.NewRepo(r.Repo, r.Name, r.Username, r.Password, []byte(r.TLSClientCAData), []byte(r.TLSClientCertData), []byte(r.TLSClientCertKey))
	default:
		return repo2.NewRepo(r.Repo, creds.GetRepoCreds(r), r.IsInsecure(), r.EnableLFS, disco.Discover)
	}
}
