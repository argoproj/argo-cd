package health

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/lua"
)

// SetApplicationHealth updates the health statuses of all resources performed in the comparison
func SetApplicationHealth(resStatuses []*appv1.ResourceStatus, liveObjs []*unstructured.Unstructured, resourceOverrides map[string]appv1.ResourceOverride, filter func(obj *unstructured.Unstructured) bool) (*appv1.HealthStatus, error) {
	var savedErr error
	appHealth := appv1.HealthStatus{Status: health.HealthStatusHealthy}
	for i, liveObj := range liveObjs {
		var healthStatus *health.HealthStatus
		var err error
		if liveObj == nil {
			healthStatus = &health.HealthStatus{Status: health.HealthStatusMissing}
		} else {
			if filter(liveObj) {
				healthStatus, err = health.GetResourceHealth(liveObj, lua.ResourceHealthOverrides(resourceOverrides))
				if err != nil && savedErr == nil {
					savedErr = err
				}
			}
		}
		if healthStatus != nil {
			resHealth := appv1.HealthStatus{Status: healthStatus.Status, Message: healthStatus.Message}
			resStatuses[i].Health = &resHealth
			ignore := ignoreLiveObjectHealth(liveObj, resHealth)
			if !ignore && health.IsWorse(appHealth.Status, healthStatus.Status) {
				appHealth.Status = healthStatus.Status
			}
		}
	}
	return &appHealth, savedErr
}

// ignoreLiveObjectHealth determines if we should not allow the live object to affect the overall
// health of the application (e.g. hooks, missing child applications)
func ignoreLiveObjectHealth(liveObj *unstructured.Unstructured, resHealth appv1.HealthStatus) bool {
	if liveObj != nil {
		// Don't allow resource hooks to affect health status
		if hookutil.IsHook(liveObj) || ignore.Ignore(liveObj) {
			return true
		}
		gvk := liveObj.GroupVersionKind()
		if gvk.Group == "argoproj.io" && gvk.Kind == "Application" && (resHealth.Status == health.HealthStatusMissing || resHealth.Status == health.HealthStatusUnknown) {
			// Covers the app-of-apps corner case where child app is deployed but that app itself
			// has a status of 'Missing', which we don't want to cause the parent's health status
			// to also be Missing
			return true
		}
	}
	return false
}
