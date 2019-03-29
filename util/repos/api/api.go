package api

type RepoType = string

type RepoCfgFactory interface {
	NormalizeURL(url string) string
	IsResolvedRevision(revision string) bool
	SameURL(leftRepo, rightRepo string) bool
}
type RepoCfg interface {
	// For a particular revision of the repository (should it have such a thing),  returns:
	// 1. a map from the app path to the app revision
	FindAppCfgs(revision string) (map[string]string, error)
	// Resolve a potentially ambigous revision of an  app to an unambiguous revision, returns:
	// 1. the unambiguous (aka "resolved") revision
	ResolveRevision(appPath string, revision string) (string, error)
	// returns:
	// 1. path on disk
	// 2. the app's type, e.g. "ksonnet", "kustomize", ""
	GetAppCfg(appPath string, resolvedRevision string) (string, string, error)
	LockKey() string
}
