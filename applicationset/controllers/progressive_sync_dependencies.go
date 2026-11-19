package controllers

import (
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/applicationset/progressivesync"
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
	logCtx *log.Entry,
	applicationSet *argov1alpha1.ApplicationSet,
	applicationStatuses []argov1alpha1.ApplicationSetApplicationStatus,
	w progressivesync.StatusWriter,
) {
	// Delegate to existing controller method
	r.setAppSetApplicationStatus(logCtx, applicationSet, applicationStatuses, w)
}

func (r *ApplicationSetReconciler) RecordProgressiveSyncTriggered(
	applicationSet *argov1alpha1.ApplicationSet,
	step string,
) {
	r.Metrics.SetProgressiveSyncAppSync(applicationSet, step)
}

func (r *ApplicationSetReconciler) SetApplicationSetStatusCondition(
	applicationSet *argov1alpha1.ApplicationSet,
	conditions []argov1alpha1.ApplicationSetCondition,
	parametersGenerated bool,
	observedGeneration int64,
	w progressivesync.StatusWriter,
) {
	// Delegate to existing controller method
	r.setApplicationSetStatusCondition(applicationSet, conditions, parametersGenerated, observedGeneration, w)
}

func (r *ApplicationSetReconciler) IncRefreshTriggeredCount(appset *argov1alpha1.ApplicationSet) {
	r.Metrics.IncRefreshTriggeredCount(appset)
}
