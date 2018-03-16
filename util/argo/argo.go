package argo

import (
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
)

// ResolveServerNamespace resolves server and namespace to use given an application spec,
// and a manifest response. It looks to explicit server/namespace overridden in the app CRD spec
// and falls back to the server/namespace defined in the ksonnet environment
func ResolveServerNamespace(destination *appv1.ApplicationDestination, manifestInfo *repository.ManifestResponse) (string, string) {
	server := manifestInfo.Server
	namespace := manifestInfo.Namespace
	if destination != nil {
		if destination.Server != "" {
			server = destination.Server
		}
		if destination.Namespace != "" {
			namespace = destination.Namespace
		}
	}
	return server, namespace
}
