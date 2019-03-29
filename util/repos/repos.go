package repos

import (
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	. "github.com/argoproj/argo-cd/util/repos/api"
)

type Registry interface {
	NewFactory(repoType RepoType) RepoCfgFactory
}

var singletonRegistry = registry{}

func GetRegistry() Registry {
	return singletonRegistry
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

func SameURL(leftUrl, rightUrl string) bool {

	return leftUrl == rightUrl ||
		singletonRegistry.NewFactory("git").SameURL(leftUrl, rightUrl) ||
		singletonRegistry.NewFactory("helm").SameURL(leftUrl, rightUrl)
}

func NormalizeURL(url string) string {

	normalizedURL := singletonRegistry.NewFactory("git").NormalizeURL(url)

	if url != normalizedURL {
		return normalizedURL
	}

	return singletonRegistry.NewFactory("helm").NormalizeURL(url)
}
