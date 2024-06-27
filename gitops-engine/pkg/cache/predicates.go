package cache

// TopLevelResource returns true if resource has no parents
func TopLevelResource(r *Resource) bool {
	return len(r.OwnerRefs) == 0
}

// ResourceOfGroupKind returns predicate that matches resource by specified group and kind
func ResourceOfGroupKind(group string, kind string) func(r *Resource) bool {
	return func(r *Resource) bool {
		key := r.ResourceKey()
		return key.Group == group && key.Kind == kind
	}
}
