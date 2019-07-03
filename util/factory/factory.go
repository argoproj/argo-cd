package factory

import (
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/depot"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
)

// ClientFactory is a factory of  Clients
// Primarily used to support creation of mock clients during unit testing
type ClientFactory interface {
	NewClient(r *v1alpha1.Repository) (depot.Client, error)
}

func NewFactory() ClientFactory {
	return &defaultClientFactory{}
}

type defaultClientFactory struct {
}

func (f *defaultClientFactory) NewClient(r *v1alpha1.Repository) (depot.Client, error) {
	switch r.Type {
	case "helm":
		return helm.NewClient(r.Repo, r.Name, r.Username, r.Password, r.CAData, r.CertData, r.KeyData)
	default:
		return git.NewClient(r.Repo, r.Username, r.Password, r.SSHPrivateKey, r.InsecureIgnoreHostKey)
	}
}
