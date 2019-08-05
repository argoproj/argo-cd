package factory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/creds"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	repoclient "github.com/argoproj/argo-cd/util/repo/client"
)

// ClientFactory is a factory of  Clients
// Primarily used to support creation of mock clients during unit testing
type ClientFactory interface {
	NewClient(r *v1alpha1.Repository) (repoclient.Client, error)
}

func NewFactory() ClientFactory {
	return &defaultClientFactory{}
}

type defaultClientFactory struct {
}

func (f *defaultClientFactory) NewClient(r *v1alpha1.Repository) (repoclient.Client, error) {
	switch r.Type {
	case "helm":
		return helm.NewClient(r.Repo, r.Name, r.Username, r.Password, []byte(r.TLSClientCAData), []byte(r.TLSClientCertData), []byte(r.TLSClientCertKey))
	default:
		return git.NewClient(r.Repo, creds.GetRepoCreds(r), r.IsInsecure(), r.EnableLFS)
	}
}
