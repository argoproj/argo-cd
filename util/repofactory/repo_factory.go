package repofactory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/creds"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/repo"
)

type RepoFactory interface {
	NewRepo(r *v1alpha1.Repository) (repo.Repo, error)
}

func NewRepoFactory() RepoFactory {
	return &repoFactory{}
}

type repoFactory struct {
}

func (f *repoFactory) NewRepo(r *v1alpha1.Repository) (repo.Repo, error) {
	switch r.Type {
	case "helm":
		return helm.NewRepo(r.Repo, r.Name, r.Username, r.Password, []byte(r.TLSClientCAData), []byte(r.TLSClientCertData), []byte(r.TLSClientCertKey))
	default:
		return git.NewRepo(r.Repo, creds.GetRepoCreds(r), r.IsInsecure(), r.EnableLFS)
	}
}
