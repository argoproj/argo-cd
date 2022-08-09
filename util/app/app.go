package app

import (
	"fmt"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// AppRBACName formats fully qualified application name for RBAC check
func AppRBACName(app appv1.Application) string {
	return fmt.Sprintf("%s/%s", app.Spec.GetProject(), app.Name)
}
