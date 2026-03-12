package manifestgen

import (
	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ResolveManifestGeneratePolicy returns the effective manifest generation policy
// using the hierarchy: app > project > global. Returns ManifestGeneratePolicyNone
// if no level specifies a policy.
func ResolveManifestGeneratePolicy(
	appPolicy *v1alpha1.ManifestGeneratePolicy,
	projectPolicy *v1alpha1.ManifestGeneratePolicy,
	globalPolicy string,
) v1alpha1.ManifestGeneratePolicy {
	if appPolicy != nil && *appPolicy != "" {
		return *appPolicy
	}
	if projectPolicy != nil && *projectPolicy != "" {
		return *projectPolicy
	}
	if globalPolicy != "" {
		return v1alpha1.ManifestGeneratePolicy(globalPolicy)
	}
	return v1alpha1.ManifestGeneratePolicyNone
}
