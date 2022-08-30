package appset

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// AppRBACName formats fully qualified application name for RBAC check
func AppSetRBACName(appSet *v1alpha1.ApplicationSet) string {
	return fmt.Sprintf("%s/%s", appSet.Spec.Template.Spec.GetProject(), appSet.ObjectMeta.Name)
}
