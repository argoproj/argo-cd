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
	// Helm details
	Helm *CustomHelmAppSpec
	// Kustomize details
	Kustomize *apiclient.KustomizeAppSpec
	// Directory details
	Directory *apiclient.DirectoryAppSpec
}

type CustomHelmAppSpec apiclient.HelmAppSpec

func (has CustomHelmAppSpec) GetParameterValueByName(Name string) string {
	var value string
	for i := range has.Parameters {
		if has.Parameters[i].Name == Name {
			value = has.Parameters[i].Value
			break
		}
	}
	return value
}

func (has CustomHelmAppSpec) GetFileParameterPathByName(Name string) string {
	var path string
	for i := range has.FileParameters {
		if has.FileParameters[i].Name == Name {
			path = has.FileParameters[i].Path
			break
		}
	}
	return path
}
