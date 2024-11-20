package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1reg "github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

func (a *Application) IsEmptyTypeMeta() bool {
	return a.TypeMeta.Size() == 0 || a.TypeMeta.Kind == "" || a.TypeMeta.APIVersion == ""
}

func (a *Application) SetDefaultTypeMeta() {
	a.TypeMeta = metav1.TypeMeta{
		Kind:       appv1reg.ApplicationKind,
		APIVersion: SchemeGroupVersion.String(),
	}
}

func (spec *ApplicationSpec) GetNonRefSource() (*ApplicationSource, int) {
	if spec.HasMultipleSources() {
		for idx, source := range spec.Sources {
			if !source.IsRef() {
				return &source, idx
			}
		}
	}

	if spec.Source == nil {
		return nil, -2
	}

	// single source app
	return spec.Source, -1
}

func (spec *ApplicationSpec) SourceUnderIdxIsHelm(idx int) bool {
	source := spec.GetSourcePtrByIndex(idx)

	return source != nil && source.IsHelm()
}
