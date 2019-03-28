package repos

import (
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	. "github.com/argoproj/argo-cd/util/repos/api"
)

type Registry interface {
	NewFactory(repoType RepoType) RepoCfgFactory
}

func NewRegistry() Registry {
	return registry{}
}

type registry struct {
}

func (r registry) NewFactory(repoType RepoType) RepoCfgFactory {
	if repoType == "helm" {
		return helm.NewRepoCfgFactory()
	} else {
		return git.NewRepoCfgFactory()
	}
}
