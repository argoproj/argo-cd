package controller

import (
	"context"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func (ctrl *ApplicationController) RollbackApp(ctx context.Context, app *appv1.Application, hydratedRevision string) error {
	return ctrl.hydrator.RollbackApp(ctx, app, hydratedRevision)
}
