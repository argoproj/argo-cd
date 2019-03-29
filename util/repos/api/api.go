package api

type RepoFactory interface {
	// Normalize a URL.
	NormalizeURL(url string) string
	// Return whether or not the URL are the same (normalized).
	SameURL(leftRepo, rightRepo string) bool
}

type Repo interface {
	// For a particular revision of the repository (should it have such a thing),  returns:
	// 1. a map from the template path to the app revision
	FindApps(revision string) (map[string]string, error)
	// Resolve a potentially ambiguous revision of a template to an unambiguous revision, returns:
	// 1. the unambiguous (aka "resolved") revision
	ResolveRevision(path string, revision string) (string, error)
	// Get the app template:
	// 1. full path of the template on disk
	// 2. the app's type, e.g. "ksonnet", "kustomize", ""
	GetTemplate(path string, resolvedRevision string) (string, string, error)
	// Return a key suitable for locking the repo. Usually the repo's URL.
	LockKey() string
}
