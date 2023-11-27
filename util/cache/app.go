package cache

// AppIdentity uniquely identifies an Application.
type AppIdentity struct {
	// name is the name of the app without any namespace prefix, i.e. metadata.name.
	name string
	// namespace is the namespace where the app resides, i.e. metadata.namespace.
	namespace string
	// defaultNamespace is the controller's default location for storing apps, i.e. the controller's metadata.namespace.
	defaultNamespace string
}

// NewAppIdentity panics if any provided string is empty. All three arguments are necessarily to construct a unique app
// identity.
func NewAppIdentity(name, namespace, defaultNamespace string) AppIdentity {
	if name == "" || namespace == "" || defaultNamespace == "" {
		panic("failed to specify all components of an app identity")
	}
	return AppIdentity{
		name:             name,
		namespace:        namespace,
		defaultNamespace: defaultNamespace,
	}
}

// QualifiedName returns the canonical app name. If the app is in the controller's namespace, it's just the app's
// metadata.name. If the app is in another namespace, it's metadata.namespace/metadata.name.
func (a AppIdentity) QualifiedName() string {
	if a.namespace == a.defaultNamespace {
		return a.name
	}
	return a.namespace + "/" + a.name
}
