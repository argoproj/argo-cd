package shared

import (
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type HelmAppSpec struct {
	Name           string
	ValueFiles     []string
	Parameters     []*v1alpha1.HelmParameter
	Values         string
	FileParameters []*v1alpha1.HelmFileParameter
}

func (has HelmAppSpec) GetParameterValueByName(Name string) string {
	var value string
	for i := range has.Parameters {
		if has.Parameters[i].Name == Name {
			value = has.Parameters[i].Value
			break
		}
	}
	return value
}

func (has HelmAppSpec) GetFileParameterPathByName(Name string) string {
	var path string
	for i := range has.FileParameters {
		if has.FileParameters[i].Name == Name {
			path = has.FileParameters[i].Path
			break
		}
	}
	return path
}
