package repos

import (
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	. "github.com/argoproj/argo-cd/util/repos/api"
)

type Registry interface {
	GetFactory(repoType string) RepoCfgFactory
}

var singletonRegistry = registry{}

func GetRegistry() Registry {
	return singletonRegistry
}

type registry struct {
}

func (r registry) GetFactory(repoType string) RepoCfgFactory {
	if repoType == "helm" {
		return helm.GetRepoCfgFactory()
	} else {
		return git.GetRepoCfgFactory()
	}
}

func SameURL(leftUrl, rightUrl string) bool {

	return leftUrl == rightUrl ||
		singletonRegistry.GetFactory("git").SameURL(leftUrl, rightUrl) ||
		singletonRegistry.GetFactory("helm").SameURL(leftUrl, rightUrl)
}

func NormalizeURL(url string) string {

	normalizedURL := singletonRegistry.GetFactory("git").NormalizeURL(url)

	if url != normalizedURL {
		return normalizedURL
	}

	return singletonRegistry.GetFactory("helm").NormalizeURL(url)
}
