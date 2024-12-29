package controller

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/health"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
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
			log.Infof("Processing CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
			// Custom logic for CRD health
			conditions, found, err := unstructured.NestedSlice(res.Live.Object, "status", "conditions")
			if err != nil {
				log.WithError(err).Warnf("Failed to retrieve conditions for CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
			}

			if found {
				log.Infof("Conditions found for CRD %s/%s: %+v", res.Live.GetNamespace(), res.Live.GetName(), conditions)
				for _, condition := range conditions {
					condMap, ok := condition.(map[string]interface{})
					if ok {
						condType, condTypeExists := condMap["type"].(string)
						log.Infof("Processing condition: %+v", condType)
						condStatus, condStatusExists := condMap["status"].(string)
						condMessage, _ := condMap["message"].(string)
						log.Infof("Condition type: %s, status: %s, message: %s", condType, condStatus, condMessage)
						if condTypeExists && condStatusExists && condType == "NonStructuralSchema" && condStatus == True {
							healthStatus = &health.HealthStatus{
								Status:  health.HealthStatusDegraded,
								Message: "CRD has non-structural schema issues",
							}
							log.Infof("Health status set to Degraded with message: %s", condMessage)
							break
						}
					} else {
						log.Warnf("Unexpected condition format for CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
					}
				}
			}
			if healthStatus == nil {
				log.Infof("Health status set to Healthy for CRD %s/%s", res.Live.GetNamespace(), res.Live.GetName())
				healthStatus = &health.HealthStatus{Status: health.HealthStatusHealthy}
			}
		} else if res.Live == nil {
			healthStatus = &health.HealthStatus{Status: health.HealthStatusMissing}
		} else {
			// App that manages itself should not affect its own health
			if isSelfReferencedApp(app, kubeutil.GetObjectRef(res.Live)) {
				continue
			}
			healthStatus, err = health.GetResourceHealth(res.Live, healthOverrides)
			if err != nil {
				errCount++
				if savedErr == nil {
					savedErr = fmt.Errorf("failed to get resource health for %q with name %q in namespace %q: %w", res.Live.GetKind(), res.Live.GetName(), res.Live.GetNamespace(), err)
				}
				// Log the error for debugging
				log.WithField("application", app.QualifiedName()).Warn(savedErr)
			}
		}

		if healthStatus == nil {
			continue
		}

		if persistResourceHealth {
			resHealth := appv1.HealthStatus{Status: healthStatus.Status, Message: healthStatus.Message}
			statuses[i].Health = &resHealth
		} else {
			statuses[i].Health = nil
		}

		// Health status checks
		if _, hasOverride := healthOverrides[lua.GetConfigMapKey(gvk)]; healthStatus.Status == health.HealthStatusMissing && !hasOverride && health.GetHealthCheckFunc(gvk) == nil {
			continue
		}

		// Ignore certain health statuses for child apps
		if res.Kind == application.ApplicationKind && res.Group == application.Group && (healthStatus.Status == health.HealthStatusMissing || healthStatus.Status == health.HealthStatusUnknown) {
			continue
		}

		if health.IsWorse(appHealth.Status, healthStatus.Status) {
			appHealth.Status = healthStatus.Status
		}
	}

	if persistResourceHealth {
		app.Status.ResourceHealthSource = appv1.ResourceHealthLocationInline
		// Update timestamp only if health status changes
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
