package api

type RepoCfgFactory interface {
	NormalizeURL(url string) string
	IsResolvedRevision(revision string) bool
	SameURL(leftRepo, rightRepo string) bool
}

type RepoCfg interface {
	// For a particular revision of the repository (should it have such a thing),  returns:
	// 1. a map from the template path to the app revision
	FindApps(revision string) (map[string]string, error)
	// Resolve a potentially ambiguous revision of a template to an unambiguous revision, returns:
	// 1. the unambiguous (aka "resolved") revision
	ResolveRevision(path string, revision string) (string, error)
	// returns:
	// 1. full path of the template on disk
	// 2. the app's type, e.g. "ksonnet", "kustomize", ""
	GetTemplate(path string, resolvedRevision string) (string, string, error)
	LockKey() string
}
