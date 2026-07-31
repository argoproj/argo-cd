package controller

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	resourceutil "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/resource"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
)

// liveObjHasExplicitSyncProtection reports whether the live object itself carries
// a Prune=false or Delete=false sync-option annotation (not app/project defaults).
func liveObjHasExplicitSyncProtection(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}
	pruneVal := resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionPrune)
	if pruneVal != nil && *pruneVal == synccommon.SyncValueFalse {
		return true
	}
	deleteVal := resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionDelete)
	if deleteVal != nil && *deleteVal == synccommon.SyncValueFalse {
		return true
	}
	return false
}

func (m *appStateManager) warnUnprotectedStrayResources(
	ctx context.Context,
	app *appv1.Application,
	logEntry *log.Entry,
	reconciliationResult sync.ReconciliationResult,
	defaultPruneOption *string,
	pruneConfirmed bool,
) {
	if m.auditLogger == nil || m.projLister == nil {
		return
	}
	eventLabels := argo.GetAppEventLabels(ctx, app, m.projLister, m.namespace, m.settingsMgr, m.db)
	for i, liveObj := range reconciliationResult.Live {
		if liveObj == nil {
			continue
		}
		var targetObj *unstructured.Unstructured
		if i < len(reconciliationResult.Target) {
			targetObj = reconciliationResult.Target[i]
		}
		if targetObj != nil {
			continue
		}
		if sync.IsPruningDisabled(liveObj, defaultPruneOption) {
			continue
		}
		if liveObjHasExplicitSyncProtection(liveObj) {
			continue
		}
		if sync.ObjRequiresPruneConfirmation(liveObj, defaultPruneOption) && !pruneConfirmed {
			continue
		}

		gvk := liveObj.GroupVersionKind()
		message := fmt.Sprintf(
			"Pruning stray resource %s/%s/%s with no Prune=false or Delete=false annotation on its live object; "+
				"if you expected it to be retained, verify the annotation was applied to this object (not only a parent manifest)",
			gvk.Group, gvk.Kind, fmt.Sprintf("%s/%s", liveObj.GetNamespace(), liveObj.GetName()),
		)
		logEntry.Warn(message)
		m.auditLogger.LogAppEvent(
			app,
			argo.EventInfo{Reason: argo.EventReasonResourceDeleted, Type: corev1.EventTypeWarning},
			message,
			"",
			eventLabels,
		)
	}
}
