package api

type RepoType = string

type RepoCfgFactory interface {
	NormalizeURL(url string) string
	IsResolvedRevision(revision string) bool
	SameURL(leftRepo, rightRepo string) bool
}
type RepoRevision = string
type RepoCfg interface {
	ListAppCfgs(revision RepoRevision) (map[AppPath]AppType, error)
	GetAppCfg(path AppPath, revision AppRevision) (string, AppType, error)
	ResolveRevision(path AppPath, revision AppRevision) (AppRevision, error)
	LockKey() string
}

type AppPath = string
type AppRevision = string
type AppType = string

const (
	HelmAppType      AppType = "helm"
	KustomizeAppType AppType = "kustomize"
	KsonnetAppType   AppType = "ksonnet"
)
