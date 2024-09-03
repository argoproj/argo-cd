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
