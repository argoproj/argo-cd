package argo

import (
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
)

// ResolveServerNamespace resolves server and namespace to use given an application spec,
// and a manifest response. It looks to explicit server/namespace overridden in the app CRD spec
// and falls back to the server/namespace defined in the ksonnet environment
func ResolveServerNamespace(app *appv1.Application, manifestInfo *repository.ManifestResponse) (string, string) {
	server := manifestInfo.Server
	namespace := manifestInfo.Namespace
	if app.Spec.Destination != nil {
		if app.Spec.Destination.Server != "" {
			server = app.Spec.Destination.Server
		}
		if app.Spec.Destination.Namespace != "" {
			namespace = app.Spec.Destination.Namespace
		}
	}
	return server, namespace
}
