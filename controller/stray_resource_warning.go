package controller

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	resourceutil "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/resource"
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

func isPruningDisabledForObj(obj *unstructured.Unstructured, defaultPruneOption *string) bool {
	var pruneOptionValue *string
	if obj != nil {
		pruneOptionValue = resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionPrune)
	}
	if pruneOptionValue == nil {
		pruneOptionValue = defaultPruneOption
	}
	return pruneOptionValue != nil && *pruneOptionValue == synccommon.SyncValueFalse
}

func (m *appStateManager) warnUnprotectedStrayResources(
	ctx context.Context,
	app *appv1.Application,
	logEntry *log.Entry,
	reconciliationResult sync.ReconciliationResult,
	defaultPruneOption *string,
) {
	if m.auditLogger == nil {
		return
	}
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
		if isPruningDisabledForObj(liveObj, defaultPruneOption) {
			continue
		}
		if liveObjHasExplicitSyncProtection(liveObj) {
			continue
		}

		gvk := liveObj.GroupVersionKind()
		message := fmt.Sprintf(
			"Pruning stray resource %s/%s/%s with no Prune=false or Delete=false annotation on its live object; "+
				"if you expected it to be retained, verify the annotation was applied to this object (not only a parent manifest)",
			gvk.Group, gvk.Kind, fmt.Sprintf("%s/%s", liveObj.GetNamespace(), liveObj.GetName()),
		)
		logEntry.Warn(message)
		eventLabels := argo.GetAppEventLabels(ctx, app, nil, m.namespace, m.settingsMgr, m.db)
		m.auditLogger.LogAppEvent(
			app,
			argo.EventInfo{Reason: argo.EventReasonResourceDeleted, Type: corev1.EventTypeWarning},
			message,
			"",
			eventLabels,
		)
	}
}
