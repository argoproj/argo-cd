package shared

import (
	"time"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

type CommitMetadata struct {
	// Commit message
	Message string
	// Commit author
	Author string
	// Commit creation date
	Date time.Time
	// Associated tags
	Tags []string
}

type AppDetail struct {
	// AppDetail Type
	Type string
	// Ksonnet details
	Ksonnet *apiclient.KsonnetAppSpec
	// Helm details
	Helm *apiclient.HelmAppSpec
	// Kustomize details
	Kustomize *apiclient.KustomizeAppSpec
	// Directory details
	Directory *apiclient.DirectoryAppSpec
}
