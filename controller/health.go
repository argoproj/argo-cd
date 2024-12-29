package controller

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/health"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/lua"
)

// setApplicationHealth updates the health statuses of all resources performed in the comparison
func setApplicationHealth(resources []managedResource, statuses []appv1.ResourceStatus, resourceOverrides map[string]appv1.ResourceOverride, app *appv1.Application, persistResourceHealth bool) (*appv1.HealthStatus, error) {
	var savedErr error
	var errCount uint

	appHealth := appv1.HealthStatus{Status: health.HealthStatusHealthy}
	for i, res := range resources {
		if res.Target != nil && hookutil.Skip(res.Target) {
			continue
		}
		if res.Target != nil && res.Target.GetAnnotations() != nil && res.Target.GetAnnotations()[common.AnnotationIgnoreHealthCheck] == "true" {
			continue
		}

		if res.Live != nil && (hookutil.IsHook(res.Live) || ignore.Ignore(res.Live)) {
			continue
		}

		var healthStatus *health.HealthStatus
		var err error
		healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)
		gvk := schema.GroupVersionKind{Group: res.Group, Version: res.Version, Kind: res.Kind}

		if res.Kind == "CustomResourceDefinition" && res.Group == "apiextensions.k8s.io" {
			// Custom logic for CRD health
			conditions, found, err := unstructured.NestedSlice(res.Live.Object, "status", "conditions")
			if err != nil {
				log.WithError(err).Warnf("Failed to retrieve conditions for CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
			}
			if found {
				for _, condition := range conditions {
					condMap, ok := condition.(map[string]interface{})
					if ok {
						condType, condTypeExists := condMap["type"].(string)
						condStatus, condStatusExists := condMap["status"].(string)
						if condTypeExists && condStatusExists && condType == "NonStructuralSchema" && condStatus == "True" {
							healthStatus = &health.HealthStatus{
								Status:  health.HealthStatusDegraded,
								Message: "CRD has non-structural schema issues",
							}
							break
						}
					} else {
						log.Warnf("Unexpected condition format for CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
					}
				}
			}
			if healthStatus == nil {
				healthStatus = &health.HealthStatus{Status: health.HealthStatusHealthy}
			}
		}

		if persistResourceHealth {
			resHealth := appv1.HealthStatus{Status: healthStatus.Status, Message: healthStatus.Message}
			statuses[i].Health = &resHealth
		} else {
			statuses[i].Health = nil
		}

		// Is health status is missing but resource has not built-in/custom health check then it should not affect parent app health
		if _, hasOverride := healthOverrides[lua.GetConfigMapKey(gvk)]; healthStatus.Status == health.HealthStatusMissing && !hasOverride && health.GetHealthCheckFunc(gvk) == nil {
			continue
		}

		// Missing or Unknown health status of child Argo CD app should not affect parent
		if res.Kind == application.ApplicationKind && res.Group == application.Group && (healthStatus.Status == health.HealthStatusMissing || healthStatus.Status == health.HealthStatusUnknown) {
			continue
		}

		if health.IsWorse(appHealth.Status, healthStatus.Status) {
			appHealth.Status = healthStatus.Status
		}
	}
	if persistResourceHealth {
		app.Status.ResourceHealthSource = appv1.ResourceHealthLocationInline
		// if the status didn't change, don't update the timestamp
		if app.Status.Health.Status == appHealth.Status && app.Status.Health.LastTransitionTime != nil {
			appHealth.LastTransitionTime = app.Status.Health.LastTransitionTime
		} else {
			now := metav1.Now()
			appHealth.LastTransitionTime = &now
		}
	} else {
		app.Status.ResourceHealthSource = appv1.ResourceHealthLocationAppTree
	}
	if savedErr != nil && errCount > 1 {
		savedErr = fmt.Errorf("see application-controller logs for %d other errors; most recent error was: %w", errCount-1, savedErr)
	}
	return &appHealth, savedErr
}
