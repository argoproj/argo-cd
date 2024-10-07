package shared

import (
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

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

type CustomHelmAppSpec struct {
	HelmAppSpec            apiclient.HelmAppSpec
	HelmParameterOverrides []v1alpha1.HelmParameter
}

func (has CustomHelmAppSpec) GetParameterValueByName(name string) string {
	// Check in overrides first
	for i := range has.HelmParameterOverrides {
		if has.HelmParameterOverrides[i].Name == name {
			return has.HelmParameterOverrides[i].Value
		}
	}

	for i := range has.HelmAppSpec.Parameters {
		if has.HelmAppSpec.Parameters[i].Name == name {
			return has.HelmAppSpec.Parameters[i].Value
		}
	}
	return ""
}

func (has CustomHelmAppSpec) GetFileParameterPathByName(name string) string {
	var path string
	for i := range has.HelmAppSpec.FileParameters {
		if has.HelmAppSpec.FileParameters[i].Name == name {
			path = has.HelmAppSpec.FileParameters[i].Path
			break
		}
	}
	return path
}
