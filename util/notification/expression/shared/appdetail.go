package shared

import "github.com/argoproj/argo-cd/v2/reposerver/apiclient"

type AppDetail struct {
	// AppDetail Type
	Type string
	// Ksonnet details
	Ksonnet *apiclient.KsonnetAppSpec
	// Helm details
	Helm *HelmAppSpec
	// Kustomize details
	Kustomize *apiclient.KustomizeAppSpec
	// Directory details
	Directory *apiclient.DirectoryAppSpec
}
