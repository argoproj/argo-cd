package controllers

import (
	"context"

	log "github.com/sirupsen/logrus"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

/**
  This file implements the progressiveSync.Dependencies interface for ApplicationSetReconciler.

  Progressive sync logic does not belong in this file. The methods here should be "bookkeeping"
  methods that keep progressive sync work in progressive sync and controller work in the controller.
  The only purpose is to provide progressive sync safe, minimal access to certain controller
  functionality to avoid duplicate code.
*/

func (r *ApplicationSetReconciler) SetAppSetApplicationStatus(
	ctx context.Context,
	logCtx *log.Entry,
	applicationSet *argov1alpha1.ApplicationSet,
	applicationStatuses []argov1alpha1.ApplicationSetApplicationStatus,
) error {
	// Delegate to existing controller method
	return r.setAppSetApplicationStatus(ctx, logCtx, applicationSet, applicationStatuses)
}

func (r *ApplicationSetReconciler) SetApplicationSetStatusCondition(
	ctx context.Context,
	applicationSet *argov1alpha1.ApplicationSet,
	condition argov1alpha1.ApplicationSetCondition,
	parametersGenerated bool,
) error {
	// Delegate to existing controller method
	return r.setApplicationSetStatusCondition(ctx, applicationSet, condition, parametersGenerated)
}
