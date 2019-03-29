package repos

import (
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	. "github.com/argoproj/argo-cd/util/repos/api"
)

func GetFactory(repoType string) RepoCfgFactory {
	if repoType == "helm" {
		return helm.GetRepoCfgFactory()
	} else {
		return git.GetRepoCfgFactory()
	}
}

func SameURL(leftUrl, rightUrl string) bool {

	return leftUrl == rightUrl ||
		GetFactory("git").SameURL(leftUrl, rightUrl) ||
		GetFactory("helm").SameURL(leftUrl, rightUrl)
}

func NormalizeURL(url string) string {

	normalizedURL := GetFactory("git").NormalizeURL(url)

	if url != normalizedURL {
		return normalizedURL
	}

	return GetFactory("helm").NormalizeURL(url)
}
