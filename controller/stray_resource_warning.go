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
	"k8s.io/apimachinery/pkg/runtime/schema"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
)

// maxStrayResourceWarningsPerSync caps the number of individual warning
// log lines and Kubernetes Events a single sync can emit for unprotected
// stray resources, so an app with many of them can't flood the API server.
const maxStrayResourceWarningsPerSync = 20

// liveObjHasPruneFalse reports whether the live object itself carries an
// explicit Prune=false sync-option annotation (not an app/project default).
// Delete=false is deliberately not treated as protection here: it only
// gates cascade delete of the whole Application (see shouldBeDeleted in
// appcontroller.go), gitops-engine's own prune path (IsPruningDisabled)
// never looks at it, so a resource with only Delete=false is still pruned
// during a normal sync even though this check would otherwise suppress the
// warning for it.
func liveObjHasPruneFalse(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}
	pruneVal := resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionPrune)
	return pruneVal != nil && *pruneVal == synccommon.SyncValueFalse
}

func (m *appStateManager) warnUnprotectedStrayResources(
	ctx context.Context,
	app *appv1.Application,
	logEntry *log.Entry,
	reconciliationResult sync.ReconciliationResult,
	defaultPruneOption *string,
	pruneConfirmed bool,
	scopedResources []appv1.SyncOperationResource,
) {
	if m.auditLogger == nil || m.projLister == nil {
		return
	}
	var eventLabels map[string]string
	warningsEmitted := 0
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
		if len(scopedResources) > 0 {
			gvk := liveObj.GroupVersionKind()
			if !argo.ContainsSyncResource(liveObj.GetName(), liveObj.GetNamespace(),
				schema.GroupVersionKind{Group: gvk.Group, Kind: gvk.Kind}, scopedResources) {
				// This sync only targets a subset of resources and gitops-engine
				// will skip pruning anything outside that selection, so warning
				// about this one would be misleading.
				continue
			}
		}
		if sync.IsPruningDisabled(liveObj, defaultPruneOption) {
			continue
		}
		if liveObjHasPruneFalse(liveObj) {
			continue
		}
		if sync.ObjRequiresPruneConfirmation(liveObj, defaultPruneOption) && !pruneConfirmed {
			continue
		}
		if warningsEmitted >= maxStrayResourceWarningsPerSync {
			logEntry.Warnf("more than %d unprotected stray resources would be pruned in this sync; "+
				"suppressing further individual warnings", maxStrayResourceWarningsPerSync)
			break
		}

		if eventLabels == nil {
			eventLabels = argo.GetAppEventLabels(ctx, app, m.projLister, m.namespace, m.settingsMgr, m.db)
		}

		gvk := liveObj.GroupVersionKind()
		message := fmt.Sprintf(
			"Pruning stray resource %s/%s/%s with no Prune=false annotation on its live object; "+
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
		warningsEmitted++
	}
}
