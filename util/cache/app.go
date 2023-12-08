package cache

// AppID uniquely identifies an Application for a given Argo CD instance. Cache functions should use this type to
// identify applications instead of strings, because we can't trust the caller to always construct a fully specified
// app name. For example, the caller may omit the namespace, in which case the cache would be polluted with multiple
// entries for the same app. Additionally, if the Set operation doesn't specify the namespace but the Get operation
// does, the cache would always return a miss.
type AppID struct {
	// name is the name of the app without any namespace prefix, i.e. metadata.name.
	name string
	// namespace is the namespace where the app resides, i.e. metadata.namespace.
	namespace string
}

// NewAppID panics if any provided string is empty. Both arguments are necessarily to construct a unique app identity.
func NewAppID(name, namespace string) AppID {
	if name == "" {
		panic("Failed to specify app name in new app identity. This is a bug. Please file an issue at https://github.com/argoproj/argo-cd")
	}
	if namespace == "" {
		panic("Failed to specify app namespace in new app identity. This is a bug. Please file an issue at https://github.com/argoproj/argo-cd")
	}
	return AppID{
		name:      name,
		namespace: namespace,
	}
}

// Key returns the canonical app name for caching purposes, i.e. metadata.namespace/metadata.name.
func (a AppID) Key() string {
	return a.namespace + "/" + a.name
}
