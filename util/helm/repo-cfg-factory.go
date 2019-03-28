package helm

import (
	"github.com/argoproj/argo-cd/util/repos/api"
)

type RepoCfgFactory struct {
}

func NewRepoCfgFactory() api.RepoCfgFactory {
	return RepoCfgFactory{}
}

func (f RepoCfgFactory) SameURL(leftRepo, rightRepo string) bool {
	return leftRepo == rightRepo
}

func (f RepoCfgFactory) NormalizeURL(url string) string {
	return url
}
